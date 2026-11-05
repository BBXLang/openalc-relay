// Package mailsync implements the minimal user-triggered INBOX sync slice
// (FORMAL-UI-MAIL-PLAN batch 3): it is the only glue between the tested
// mailadapter (IMAP dial, snapshot, UID range fetch, MIME parse) and the
// encrypted mailstore (deduplicating import, sealed fields, sync anchor).
//
// Deliberately out of scope, per the plan: background polling, retry and
// backoff, sending, attachments download and multi-folder support. Every
// sync is an explicit request with a bounded fetch window, errors are
// reported honestly (a failed round never reports success and never
// advances the anchor), and no credential, subject or body is ever logged.
package mailsync

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/OpenALC/relay/server/internal/mailadapter"
	"github.com/OpenALC/relay/server/internal/mailstore"
	"github.com/OpenALC/relay/server/internal/secrets"
)

// Classified sync failures. They wrap the underlying cause and are the only
// errors HTTP handlers may translate into stable problem codes; the wrapped
// detail must never reach a response body or a log line.
var (
	// ErrConnect covers dial, TLS and session establishment failures.
	ErrConnect = errors.New("mail server connection failed")
	// ErrAuth covers credentials rejected by the remote server.
	ErrAuth = errors.New("mail server authentication failed")
	// ErrFetch covers snapshot-time fetch failures inside an established session.
	ErrFetch = errors.New("mail server fetch failed")
	// ErrPolicy covers endpoint-policy denials (port whitelist, private or
	// loopback targets without the administrator allowlist hook, unresolvable
	// hosts). Refused before any connection is made.
	ErrPolicy = errors.New("mail endpoint denied by policy")
)

// FolderInbox is the only folder the minimal slice imports.
const FolderInbox = "INBOX"

// SyncBatchLimit bounds one sync round: at most this many messages are
// fetched and imported per call. A backlog beyond the limit is picked up by
// subsequent syncs; the anchor only ever advances to what this round
// actually processed.
const SyncBatchLimit = 200

// SyncResult is the controlled outcome of one sync round.
type SyncResult struct {
	// UIDValidity is the INBOX UIDVALIDITY the round ran against.
	UIDValidity uint32 `json:"uid_validity"`
	// FetchedNew counts messages newly stored by this round.
	FetchedNew int `json:"fetched_new"`
	// Duplicates counts fetched messages whose (account, folder,
	// remote_message_id) key already existed with identical content.
	Duplicates int `json:"duplicates"`
	// Conflicts counts fetched messages whose key exists with different
	// content; the stored record is kept and never overwritten.
	Conflicts int `json:"conflicts"`
	// SkippedMalformed counts fetched messages that could not be imported
	// as a usable record — unparsable MIME or content the store rejected
	// as invalid (for example a body beyond the storage bound). They are
	// skipped, not stored, and the round continues (and advances past
	// them) instead of failing.
	SkippedMalformed int `json:"skipped_malformed"`
	// HighestUID is the anchor persisted by this round: the highest UID
	// processed so far for the current UIDVALIDITY. The next unchanged-
	// validity sync fetches strictly above it.
	HighestUID uint32 `json:"highest_uid"`
}

// Syncer runs user-triggered syncs for owner-scoped mail accounts. Store and
// Keys are the same encrypted-storage dependencies the HTTP layer already
// holds; SyncInbox dials only endpoints the passed policy allows. RootCAs
// swaps the trusted root store for private-CA mail hosts (nil uses the
// system roots); certificate and hostname verification itself is never
// skipped, matching the adapter contract.
type Syncer struct {
	Store   *mailstore.Store
	Keys    *secrets.Keyring
	RootCAs *x509.CertPool
}

// SyncInbox performs one bounded INBOX sync round for the account:
//
//  1. Read the owner-scoped account, decrypt its credential (mailstore
//     enforces scope; revoked accounts are refused).
//  2. Dial the IMAP endpoint through the policy (policy denies fail before
//     any connection).
//  3. Snapshot UIDVALIDITY and the server-side highest UID.
//  4. Decide the fetch window: UIDVALIDITY changed or no stored anchor ->
//     bounded full pull from UID 1; otherwise fetch strictly above the
//     stored watermark.
//  5. Fetch, parse and import each message through the store's
//     deduplicating path; parse failures are skipped and counted, storage
//     failures abort the whole round with an error.
//  6. Persist the new anchor and successful-sync time only after the
//     imports committed.
//
// A returned error always means nothing reported below happened: the anchor
// is untouched and the next sync re-covers the round honestly.
func (s *Syncer) SyncInbox(ctx context.Context, ownerID, accountID string, policy mailadapter.EndpointPolicy) (SyncResult, error) {
	if s == nil || s.Store == nil || s.Store.DB == nil || s.Store.Keys == nil {
		return SyncResult{}, errors.New("mail sync is not configured")
	}
	account, credential, anchor, err := s.Store.OpenSyncState(ctx, ownerID, accountID)
	if err != nil {
		return SyncResult{}, err
	}
	client, err := s.dial(ctx, account, credential, policy)
	if err != nil {
		return SyncResult{}, err
	}
	defer client.Close()
	snapshot := client.Snapshot()

	fromUID := uint32(1)
	if anchor.Set && anchor.UIDValidity == snapshot.UIDValidity {
		// Same mailbox generation: fetch strictly above the processed
		// watermark. The wrap guard keeps a (theoretical) full watermark
		// from restarting at UID 0.
		if anchor.HighestUID == ^uint32(0) {
			fromUID = snapshot.UIDNext
		} else {
			fromUID = anchor.HighestUID + 1
		}
	}
	var fetched []mailadapter.FetchedMessage
	if snapshot.HighestUID > 0 && snapshot.HighestUID >= fromUID {
		fetched, err = client.FetchUIDRange(ctx, fromUID, snapshot.HighestUID, SyncBatchLimit)
		if err != nil {
			return SyncResult{}, fmt.Errorf("%w: %v", ErrFetch, err)
		}
	}

	inputs := make([]mailstore.MessageInput, 0, len(fetched))
	skipped := 0
	highestProcessed := uint32(0)
	if anchor.Set && anchor.UIDValidity == snapshot.UIDValidity {
		highestProcessed = anchor.HighestUID
	}
	for _, message := range fetched {
		if message.UID > highestProcessed {
			highestProcessed = message.UID
		}
		input, ok := syncInput(account.ID, message)
		if !ok {
			skipped++
			continue
		}
		inputs = append(inputs, input)
	}
	var report mailstore.ImportReport
	if len(inputs) > 0 {
		report, err = s.Store.ImportMessages(ctx, ownerID, inputs, time.Now())
		if err != nil {
			// Storage failure: abort with an error and leave the anchor
			// untouched so the next sync re-imports the round.
			return SyncResult{}, err
		}
	}
	if err := s.Store.SetSyncAnchor(ctx, ownerID, accountID, mailstore.SyncAnchor{
		UIDValidity: snapshot.UIDValidity, HighestUID: highestProcessed, Set: true,
	}, time.Now()); err != nil {
		return SyncResult{}, err
	}
	return SyncResult{
		UIDValidity: snapshot.UIDValidity,
		FetchedNew:  report.Accepted,
		Duplicates:  report.Duplicates,
		Conflicts:   report.Conflicts,
		// Both skip stages (unparsable MIME and store-rejected records)
		// report in the same bounded-skip bucket: neither is a sync
		// failure, and both advance the watermark past the record.
		SkippedMalformed: skipped + report.Rejected,
		HighestUID:       highestProcessed,
	}, nil
}

// TestConnection verifies the stored credential against the configured IMAP
// endpoint: dial, snapshot, close. It is a pure probe — nothing in storage
// changes, not even the connection state or the sync anchor.
func (s *Syncer) TestConnection(ctx context.Context, ownerID, accountID string, policy mailadapter.EndpointPolicy) error {
	if s == nil || s.Store == nil || s.Store.DB == nil || s.Store.Keys == nil {
		return errors.New("mail sync is not configured")
	}
	account, credential, _, err := s.Store.OpenSyncState(ctx, ownerID, accountID)
	if err != nil {
		return err
	}
	client, err := s.dial(ctx, account, credential, policy)
	if err != nil {
		return err
	}
	// Snapshot is captured at SELECT time inside Dial; reaching this point
	// already proves the session is established on INBOX.
	return client.Close()
}

// dial builds the adapter config from the account row and classifies every
// dial failure into exactly one sync sentinel. The credential never appears
// in any returned error.
func (s *Syncer) dial(ctx context.Context, account mailstore.Account, credential string, policy mailadapter.EndpointPolicy) (mailadapter.IMAPClient, error) {
	cfg := mailadapter.IMAPConfig{
		Host:        account.IMAPHost,
		Port:        account.IMAPPort,
		TLS:         mailadapter.TLSPreference(account.TLSMode),
		Username:    account.Username,
		AppPassword: credential,
		RootCAs:     s.RootCAs,
	}
	client, err := mailadapter.Dial(ctx, cfg, policy)
	if err == nil {
		return client, nil
	}
	switch {
	case errors.Is(err, mailadapter.ErrPrivateTarget),
		errors.Is(err, mailadapter.ErrPortDenied),
		errors.Is(err, mailadapter.ErrHostInvalid),
		errors.Is(err, mailadapter.ErrUnresolvable):
		return nil, fmt.Errorf("%w: %v", ErrPolicy, err)
	case mailadapter.IsAuthFailure(err):
		return nil, fmt.Errorf("%w: %v", ErrAuth, err)
	default:
		// Config validation, dial refusal, TLS handshake and SELECT
		// failures are all connect-time failures.
		return nil, fmt.Errorf("%w: %v", ErrConnect, err)
	}
}

// syncInput converts one fetched message into a store input. Fetched raw
// bytes are untrusted remote input and are only ever handed to ParseMIME. A
// message that cannot be parsed yields ok=false and is skipped and counted,
// never stored and never treated as a sync failure.
func syncInput(accountID string, message mailadapter.FetchedMessage) (mailstore.MessageInput, bool) {
	parsed, err := mailadapter.ParseMIME(message.Raw)
	if err != nil {
		// Degraded parses are skipped wholesale in this slice: a partially
		// recovered record must not masquerade as a fully imported one.
		return mailstore.MessageInput{}, false
	}
	if parsed.From == nil || len(parsed.From) == 0 {
		return mailstore.MessageInput{}, false
	}
	if message.RemoteMessageID == "" {
		return mailstore.MessageInput{}, false
	}
	body := parsed.TextBody
	format := mailstore.BodyTextPlain
	if body == "" && parsed.HTMLBody != "" {
		// Stored as the sanitized-html format with the verbatim HTML for
		// the HTTP layer's documented unsanitized interim behavior; no
		// rendering decision happens here.
		body = parsed.HTMLBody
		format = mailstore.BodySanitizedHTML
	}
	received := message.InternalDate
	if received.IsZero() {
		received = parsed.Date
	}
	if received.IsZero() {
		// No usable date anywhere: the store requires a positive
		// received_at, so anchor the record at the import time.
		received = time.Now()
	}
	headers := mailstore.MessageHeaders{From: parsed.From[0].String()}
	for _, address := range parsed.To {
		headers.To = append(headers.To, address.String())
	}
	for _, address := range parsed.Cc {
		headers.Cc = append(headers.Cc, address.String())
	}
	return mailstore.MessageInput{
		AccountID:       accountID,
		Folder:          FolderInbox,
		RemoteMessageID: message.RemoteMessageID,
		ReceivedAt:      received,
		SizeBytes:       int64(len(message.Raw)),
		Unread:          true,
		HasAttachments:  false,
		BodyFormat:      format,
		Subject:         parsed.Subject,
		Headers:         headers,
		Body:            body,
		AttachmentCount: 0,
	}, true
}
