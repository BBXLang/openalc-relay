// Sync-slice additions to the encrypted mail store (FORMAL-UI-MAIL-PLAN
// batch 3). These methods exist only to serve internal/mailsync: they open
// the sealed IMAP credential for dialing, persist the incremental-sync
// anchor and import fetched messages through the existing deduplicating
// InsertMessage path. No existing method semantics are changed.

package mailstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/OpenALC/relay/server/internal/audit"
	"github.com/OpenALC/relay/server/internal/secrets"
)

// SyncAnchor is the persisted incremental-sync watermark for one account's
// INBOX. Set reports whether a previous sync recorded an anchor; when it is
// false the next sync performs a bounded full pull.
type SyncAnchor struct {
	UIDValidity uint32
	HighestUID  uint32
	Set         bool
}

// ImportReport counts the outcomes of one ImportMessages batch.
type ImportReport struct {
	// Accepted is the number of newly stored messages.
	Accepted int
	// Duplicates counts repeated imports of an identical stored record;
	// the stored row is left untouched.
	Duplicates int
	// Conflicts counts imports whose (account, folder, remote_message_id)
	// key exists with different decrypted content; the stored record is
	// kept and the conflict is reported, never overwritten.
	Conflicts int
	// Rejected counts messages the store refused as invalid input (for
	// example a body beyond the storage bound). They are skipped, never
	// stored, and must not block the batch or the sync anchor: one hostile
	// or oversized message must not deadlock every future round.
	Rejected int
}

// OpenSyncState reads one owner-scoped account together with its decrypted
// IMAP credential and the stored sync anchor. The credential exists only to
// hand to the sync layer's dial; it must never be logged, echoed or stored
// anywhere else. A missing account is ErrNotFound, a revoked account
// ErrDenied; decryption failures are ErrCiphertext and never fall back to a
// zero-value credential.
func (s Store) OpenSyncState(ctx context.Context, ownerID, accountID string) (Account, string, SyncAnchor, error) {
	var (
		account     Account
		credential  []byte
		lastSyncMs  sql.NullInt64
		revokedMs   sql.NullInt64
		createdMs   int64
		uidValidity sql.NullInt64
		highestUID  sql.NullInt64
	)
	err := s.DB.QueryRowContext(ctx, `SELECT account_id, address, imap_host, imap_port,
		smtp_host, smtp_port, tls_mode, username, connection_state, last_sync_ok_at_ms,
		revoked_at_ms, created_at_ms, credential_ciphertext, sync_uidvalidity,
		sync_highest_uid FROM mail_accounts
		WHERE owner_id = ? AND account_id = ?`, ownerID, accountID).
		Scan(&account.ID, &account.Address, &account.IMAPHost, &account.IMAPPort,
			&account.SMTPHost, &account.SMTPPort, &account.TLSMode, &account.Username,
			&account.ConnectionState, &lastSyncMs, &revokedMs, &createdMs,
			&credential, &uidValidity, &highestUID)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, "", SyncAnchor{}, ErrNotFound
	}
	if err != nil {
		return Account{}, "", SyncAnchor{}, fmt.Errorf("read mail sync state: %w", err)
	}
	if lastSyncMs.Valid {
		at := time.UnixMilli(lastSyncMs.Int64).UTC()
		account.LastSyncAt = &at
	}
	account.Revoked = revokedMs.Valid
	account.CreatedAt = time.UnixMilli(createdMs).UTC()
	if account.Revoked {
		return Account{}, "", SyncAnchor{}, ErrDenied
	}
	aad, err := secrets.AAD(secrets.DomainMailAccount, ownerID, accountID, accountID, "credential")
	if err != nil {
		return Account{}, "", SyncAnchor{}, err
	}
	plain, err := s.Keys.OpenFromBlob(aad, credential)
	if err != nil {
		return Account{}, "", SyncAnchor{}, fmt.Errorf("%w: mail credential: %v", ErrCiphertext, err)
	}
	anchor := SyncAnchor{Set: uidValidity.Valid && highestUID.Valid}
	if anchor.Set {
		if uidValidity.Int64 < 0 || uidValidity.Int64 > 0xFFFFFFFF ||
			highestUID.Int64 < 0 || highestUID.Int64 > 0xFFFFFFFF {
			return Account{}, "", SyncAnchor{}, fmt.Errorf("%w: mail sync anchor out of range", ErrInvalid)
		}
		anchor.UIDValidity = uint32(uidValidity.Int64)
		anchor.HighestUID = uint32(highestUID.Int64)
	}
	return account, string(plain), anchor, nil
}

// ImportMessages stores a batch of fetched messages by delegating each one
// to InsertMessage, which owns the single-message transaction, the sealed
// fields and the (account_id, folder, remote_message_id) deduplication. A
// same-key-different-content conflict counts towards the report and never
// aborts the batch; a message that fails input validation is likewise
// skipped and counted. Any other failure (storage, keying, ciphertext,
// revoked account) aborts immediately and is returned so the caller can
// leave the sync anchor unadvanced.
func (s Store) ImportMessages(ctx context.Context, ownerID string, inputs []MessageInput, now time.Time) (ImportReport, error) {
	var report ImportReport
	for _, input := range inputs {
		status, err := s.InsertMessage(ctx, ownerID, input, now)
		switch {
		case err == nil && status == StatusDuplicate:
			report.Duplicates++
		case err == nil:
			report.Accepted++
		case errors.Is(err, ErrConflict):
			report.Conflicts++
		case errors.Is(err, ErrInvalid):
			report.Rejected++
		case errors.Is(err, ErrDenied):
			// The account was revoked mid-batch: stop without pretending
			// the rest was imported.
			return report, err
		default:
			return report, err
		}
	}
	return report, nil
}

// SetSyncAnchor persists the processed INBOX watermark and the successful
// sync time in one transaction, guarded against revoked accounts. It must
// only be called after the round's imports committed; a failure here is a
// sync failure and the caller must not report success.
func (s Store) SetSyncAnchor(ctx context.Context, ownerID, accountID string, anchor SyncAnchor, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mail anchor update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE mail_accounts SET
		sync_uidvalidity = ?, sync_highest_uid = ?, connection_state = ?,
		last_sync_ok_at_ms = ?
		WHERE owner_id = ? AND account_id = ? AND revoked_at_ms IS NULL`,
		anchor.UIDValidity, anchor.HighestUID, ConnectionOK,
		now.UTC().UnixMilli(), ownerID, accountID)
	if err != nil {
		return fmt.Errorf("record mail sync anchor: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return err
		}
		return ErrNotFound
	}
	auditor := audit.Store{DB: tx}
	if err := auditor.Append(ctx, audit.Entry{
		OwnerID: ownerID, ActorKind: audit.ActorSystem, ActorID: "mail-sync",
		Action: "mail.account.sync", SubjectKind: "mail_account", SubjectID: accountID,
		Detail: map[string]string{
			"uid_validity": strconv.FormatUint(uint64(anchor.UIDValidity), 10),
			"highest_uid":  strconv.FormatUint(uint64(anchor.HighestUID), 10),
		},
	}, now); err != nil {
		return fmt.Errorf("audit mail sync anchor: %w", err)
	}
	return tx.Commit()
}
