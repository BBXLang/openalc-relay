// Package mailstore implements encrypted storage for mail accounts, messages
// and attachments. All queries enforce owner (and account) scope, payloads are
// sealed through the keyring with per-field AAD, lists use stable keyset
// cursors, imports deduplicate on (account_id, folder, remote_message_id) and
// search decrypts only the authorized scope.
package mailstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/OpenALC/relay/server/internal/audit"
	"github.com/OpenALC/relay/server/internal/cursor"
	"github.com/OpenALC/relay/server/internal/randid"
	"github.com/OpenALC/relay/server/internal/secrets"
)

var (
	ErrNotFound   = errors.New("resource not found")
	ErrConflict   = errors.New("resource already used")
	ErrDenied     = errors.New("resource denied")
	ErrCiphertext = errors.New("encrypted payload rejected")
	ErrInvalid    = errors.New("request invalid")
)

// Import result statuses.
const (
	StatusAccepted  = "accepted"
	StatusDuplicate = "duplicate"
)

// Body formats mirrored by the schema CHECK.
const (
	BodyTextPlain     = "text_plain"
	BodySanitizedHTML = "sanitized_html"
)

// Attachment download states mirrored by the schema CHECK.
const (
	DownloadRemoteOnly = "remote_only"
	DownloadStored     = "stored"
)

// Account TLS modes and connection states mirrored by the schema CHECK.
const (
	TLSStartTLS    = "starttls"
	TLSImplicitTLS = "implicit_tls"

	ConnectionUntested = "untested"
	ConnectionOK       = "ok"
	ConnectionFailed   = "failed"
)

const (
	// maxAttachmentBytes is the hard storage bound; the configurable per
	// message default (10 MiB) is enforced by the calling layer.
	maxAttachmentBytes = 64 << 20
	maxBodyBytes       = 1 << 20
	defaultPageSize    = 20
	maxPageSize        = 100
	maxSearchScan      = 500
	previewRunes       = 120
)

type Store struct {
	DB   *sql.DB
	Keys *secrets.Keyring
}

// Account never exposes credentials; they are write-only and revocable.
type Account struct {
	ID              string     `json:"account_id"`
	Address         string     `json:"address"`
	IMAPHost        string     `json:"imap_host"`
	IMAPPort        int        `json:"imap_port"`
	SMTPHost        string     `json:"smtp_host"`
	SMTPPort        int        `json:"smtp_port"`
	TLSMode         string     `json:"tls_mode"`
	Username        string     `json:"username"`
	ConnectionState string     `json:"connection_state"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	Revoked         bool       `json:"revoked"`
	CreatedAt       time.Time  `json:"created_at"`
}

type CreateAccountInput struct {
	Address  string
	IMAPHost string
	IMAPPort int
	SMTPHost string
	SMTPPort int
	TLSMode  string
	Username string
	Password string
}

type MessageHeaders struct {
	From string   `json:"from"`
	To   []string `json:"to"`
	Cc   []string `json:"cc,omitempty"`
}

type MessageInput struct {
	AccountID       string
	Folder          string
	RemoteMessageID string
	ReceivedAt      time.Time
	SizeBytes       int64
	Unread          bool
	HasAttachments  bool
	BodyFormat      string
	Subject         string
	Headers         MessageHeaders
	Body            string
	AttachmentCount int
}

// Message is the decrypted detail view of one stored mail message.
type Message struct {
	ID              string         `json:"message_id"`
	AccountID       string         `json:"account_id"`
	Folder          string         `json:"folder"`
	RemoteMessageID string         `json:"remote_message_id"`
	ReceivedAt      time.Time      `json:"received_at"`
	SizeBytes       int64          `json:"size_bytes"`
	Unread          bool           `json:"unread"`
	HasAttachments  bool           `json:"has_attachments"`
	BodyFormat      string         `json:"body_format"`
	Subject         string         `json:"subject"`
	Headers         MessageHeaders `json:"headers"`
	Body            string         `json:"body"`
	AttachmentCount int            `json:"attachment_count"`
	CreatedAt       time.Time      `json:"created_at"`
}

// MessageSummary is the decrypted list view: subject, sender and a bounded
// body preview. Content is decrypted per page, never indexed in plaintext.
type MessageSummary struct {
	ID              string    `json:"message_id"`
	AccountID       string    `json:"account_id"`
	Folder          string    `json:"folder"`
	ReceivedAt      time.Time `json:"received_at"`
	Unread          bool      `json:"unread"`
	HasAttachments  bool      `json:"has_attachments"`
	AttachmentCount int       `json:"attachment_count"`
	Subject         string    `json:"subject"`
	From            string    `json:"from"`
	Preview         string    `json:"preview"`
}

type ListOptions struct {
	AccountID string
	Folder    string
	Cursor    string
	Limit     int
}

type MessagePage struct {
	Messages   []MessageSummary `json:"messages"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type SearchOptions struct {
	AccountID string
	Folder    string
	Query     string
	Cursor    string
	Limit     int
}

type AttachmentMeta struct {
	ID            string    `json:"attachment_id"`
	MessageID     string    `json:"message_id"`
	AccountID     string    `json:"account_id"`
	Filename      string    `json:"filename"`
	MimeType      string    `json:"mime_type"`
	SizeBytes     int64     `json:"size_bytes"`
	DownloadState string    `json:"download_state"`
	CreatedAt     time.Time `json:"created_at"`
}

type AttachmentInput struct {
	Filename  string
	MimeType  string
	SizeBytes int64
}

// CreateAccount stores a new account. The credential is sealed under the
// account AAD domain and never returned; inputs are validated before any write.
func (s Store) CreateAccount(ctx context.Context, ownerID string, input CreateAccountInput, now time.Time) (Account, error) {
	if !validAddress(input.Address) || !printable(input.IMAPHost, 1, 253) ||
		!printable(input.SMTPHost, 1, 253) || !printable(input.Username, 1, 254) ||
		input.IMAPPort < 1 || input.IMAPPort > 65535 || input.SMTPPort < 1 || input.SMTPPort > 65535 ||
		input.TLSMode != TLSStartTLS && input.TLSMode != TLSImplicitTLS ||
		!freeText(input.Password, 1, 4096) {
		return Account{}, ErrInvalid
	}
	accountID, err := randid.New()
	if err != nil {
		return Account{}, err
	}
	aad, err := secrets.AAD(secrets.DomainMailAccount, ownerID, accountID, accountID, "credential")
	if err != nil {
		return Account{}, err
	}
	credential, err := s.Keys.SealToBlob(aad, []byte(input.Password))
	if err != nil {
		return Account{}, fmt.Errorf("seal mail credential: %w", err)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Account{}, fmt.Errorf("begin account creation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO mail_accounts
		(account_id, owner_id, address, imap_host, imap_port, smtp_host, smtp_port, tls_mode,
		 username, credential_ciphertext, connection_state, created_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID, ownerID, input.Address, input.IMAPHost, input.IMAPPort,
		input.SMTPHost, input.SMTPPort, input.TLSMode, input.Username,
		credential, ConnectionUntested, now.UTC().UnixMilli()); err != nil {
		return Account{}, fmt.Errorf("create mail account: %w", err)
	}
	auditor := audit.Store{DB: tx}
	if err := auditor.Append(ctx, audit.Entry{
		OwnerID: ownerID, ActorKind: audit.ActorOwnerSession, ActorID: ownerID,
		Action: "mail.account.create", SubjectKind: "mail_account", SubjectID: accountID,
		Detail: map[string]string{"connection_state": ConnectionUntested},
	}, now); err != nil {
		return Account{}, fmt.Errorf("audit account creation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Account{}, fmt.Errorf("commit account creation: %w", err)
	}
	account, err := s.GetAccount(ctx, ownerID, accountID)
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

const accountSelect = `SELECT account_id, address, imap_host, imap_port,
	smtp_host, smtp_port, tls_mode, username, connection_state, last_sync_ok_at_ms,
	revoked_at_ms, created_at_ms FROM mail_accounts`

// GetAccount returns account metadata without credentials.
func (s Store) GetAccount(ctx context.Context, ownerID, accountID string) (Account, error) {
	account, err := scanAccount(singleRow{s.DB.QueryRowContext(ctx,
		accountSelect+" WHERE owner_id = ? AND account_id = ?", ownerID, accountID)})
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("read mail account: %w", err)
	}
	return account, nil
}

type singleRow struct{ row *sql.Row }

func (r singleRow) Scan(dest ...any) error { return r.row.Scan(dest...) }

func scanAccount(scanner interface{ Scan(dest ...any) error }) (Account, error) {
	var account Account
	var lastSyncMs, revokedMs sql.NullInt64
	var createdMs int64
	if err := scanner.Scan(&account.ID, &account.Address, &account.IMAPHost, &account.IMAPPort,
		&account.SMTPHost, &account.SMTPPort, &account.TLSMode, &account.Username,
		&account.ConnectionState, &lastSyncMs, &revokedMs, &createdMs); err != nil {
		return Account{}, err
	}
	if lastSyncMs.Valid {
		at := time.UnixMilli(lastSyncMs.Int64).UTC()
		account.LastSyncAt = &at
	}
	account.Revoked = revokedMs.Valid
	account.CreatedAt = time.UnixMilli(createdMs).UTC()
	return account, nil
}

// ListAccounts returns the owner's accounts, newest first. Rows are scanned
// directly: a nested query would deadlock on the single-connection pool.
func (s Store) ListAccounts(ctx context.Context, ownerID string) ([]Account, error) {
	rows, err := s.DB.QueryContext(ctx, accountSelect+`
		WHERE owner_id = ? ORDER BY created_at_ms DESC, account_id DESC LIMIT 100`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list mail accounts: %w", err)
	}
	defer rows.Close()
	accounts := make([]Account, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scan mail account: %w", err)
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

// SetConnectionResult records a connection test outcome and, on success, the
// most recent successful sync time.
func (s Store) SetConnectionResult(ctx context.Context, ownerID, accountID string, ok bool, now time.Time) error {
	state := ConnectionFailed
	syncMs := any(nil)
	if ok {
		state = ConnectionOK
		syncMs = now.UTC().UnixMilli()
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE mail_accounts SET connection_state = ?,
		last_sync_ok_at_ms = COALESCE(?, last_sync_ok_at_ms)
		WHERE owner_id = ? AND account_id = ?`, state, syncMs, ownerID, accountID)
	if err != nil {
		return fmt.Errorf("record mail connection result: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return err
		}
		return ErrNotFound
	}
	return nil
}

// RevokeAccount stops synchronization for the account immediately.
func (s Store) RevokeAccount(ctx context.Context, ownerID, accountID string, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin account revocation: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE mail_accounts SET revoked_at_ms = ?
		WHERE owner_id = ? AND account_id = ? AND revoked_at_ms IS NULL`,
		now.UTC().UnixMilli(), ownerID, accountID)
	if err != nil {
		return fmt.Errorf("revoke mail account: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return err
		}
		return ErrNotFound
	}
	auditor := audit.Store{DB: tx}
	if err := auditor.Append(ctx, audit.Entry{
		OwnerID: ownerID, ActorKind: audit.ActorOwnerSession, ActorID: ownerID,
		Action: "mail.account.revoke", SubjectKind: "mail_account", SubjectID: accountID,
	}, now); err != nil {
		return fmt.Errorf("audit account revocation: %w", err)
	}
	return tx.Commit()
}

// DeleteAccount removes the account and cascades to its messages.
func (s Store) DeleteAccount(ctx context.Context, ownerID, accountID string, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin account deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM mail_accounts
		WHERE owner_id = ? AND account_id = ?`, ownerID, accountID)
	if err != nil {
		return fmt.Errorf("delete mail account: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return err
		}
		return ErrNotFound
	}
	auditor := audit.Store{DB: tx}
	if err := auditor.Append(ctx, audit.Entry{
		OwnerID: ownerID, ActorKind: audit.ActorOwnerSession, ActorID: ownerID,
		Action: "mail.account.delete", SubjectKind: "mail_account", SubjectID: accountID,
	}, now); err != nil {
		return fmt.Errorf("audit account deletion: %w", err)
	}
	return tx.Commit()
}

// InsertMessage stores one imported message in a single transaction. A repeated
// (account_id, folder, remote_message_id) with identical decrypted content
// returns StatusDuplicate without re-inserting; the same key with different
// content fails with ErrConflict and leaves the stored record untouched.
func (s Store) InsertMessage(ctx context.Context, ownerID string, input MessageInput, now time.Time) (string, error) {
	if !freeText(input.Folder, 1, 256) || !freeText(input.RemoteMessageID, 1, 998) ||
		input.BodyFormat != BodyTextPlain && input.BodyFormat != BodySanitizedHTML ||
		!freeText(input.Subject, 0, 998) || !freeText(input.Body, 0, maxBodyBytes) ||
		input.SizeBytes < 0 || input.AttachmentCount < 0 || input.AttachmentCount > 1000 ||
		input.ReceivedAt.IsZero() || !printable(input.Headers.From, 1, 254) {
		return "", ErrInvalid
	}
	if len(input.Headers.To) > 64 || len(input.Headers.Cc) > 64 {
		return "", ErrInvalid
	}
	for _, address := range append(append([]string{}, input.Headers.To...), input.Headers.Cc...) {
		if !printable(address, 3, 254) {
			return "", ErrInvalid
		}
	}
	headersJSON, err := json.Marshal(input.Headers)
	if err != nil {
		return "", fmt.Errorf("encode mail headers: %w", err)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin mail import: %w", err)
	}
	defer tx.Rollback()
	var revoked int
	err = tx.QueryRowContext(ctx, `SELECT revoked_at_ms IS NOT NULL FROM mail_accounts
		WHERE account_id = ? AND owner_id = ?`, input.AccountID, ownerID).Scan(&revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("check mail account: %w", err)
	}
	if revoked == 1 {
		return "", ErrDenied
	}
	messageID, err := randid.New()
	if err != nil {
		return "", err
	}
	subject, err := sealField(s.Keys, secrets.DomainMailMessage, ownerID, input.AccountID, messageID, "subject", []byte(input.Subject))
	if err != nil {
		return "", err
	}
	headers, err := sealField(s.Keys, secrets.DomainMailMessage, ownerID, input.AccountID, messageID, "headers", headersJSON)
	if err != nil {
		return "", err
	}
	body, err := sealField(s.Keys, secrets.DomainMailMessage, ownerID, input.AccountID, messageID, "body", []byte(input.Body))
	if err != nil {
		return "", err
	}
	inserted, err := tx.ExecContext(ctx, `INSERT INTO mail_messages
		(message_id, owner_id, account_id, folder, remote_message_id, received_at_ms,
		 size_bytes, is_unread, has_attachments, body_format, subject_ciphertext,
		 headers_ciphertext, body_ciphertext, attachment_count, created_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, folder, remote_message_id) DO NOTHING`,
		messageID, ownerID, input.AccountID, input.Folder, input.RemoteMessageID,
		input.ReceivedAt.UTC().UnixMilli(), input.SizeBytes, boolInt(input.Unread),
		boolInt(input.HasAttachments), input.BodyFormat, subject, headers, body,
		input.AttachmentCount, now.UTC().UnixMilli())
	if err != nil {
		return "", fmt.Errorf("store mail message: %w", err)
	}
	count, err := inserted.RowsAffected()
	if err != nil {
		return "", err
	}
	if count == 1 {
		auditor := audit.Store{DB: tx}
		if err := auditor.Append(ctx, audit.Entry{
			OwnerID: ownerID, ActorKind: audit.ActorSystem, ActorID: "mail-sync",
			Action: "mail.message.import", SubjectKind: "mail_message", SubjectID: messageID,
			Detail: map[string]string{"account_id": input.AccountID, "status": StatusAccepted},
		}, now); err != nil {
			return "", fmt.Errorf("audit mail import: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("commit mail import: %w", err)
		}
		return StatusAccepted, nil
	}
	// Duplicate key: fail closed unless the decrypted content matches exactly.
	var storedID string
	var storedFormat string
	var storedSubject, storedHeaders, storedBody []byte
	err = tx.QueryRowContext(ctx, `SELECT message_id, body_format, subject_ciphertext,
		headers_ciphertext, body_ciphertext FROM mail_messages
		WHERE account_id = ? AND folder = ? AND remote_message_id = ?`,
		input.AccountID, input.Folder, input.RemoteMessageID).
		Scan(&storedID, &storedFormat, &storedSubject, &storedHeaders, &storedBody)
	if err != nil {
		return "", fmt.Errorf("read stored mail message: %w", err)
	}
	storedSubjectPlain, err := openField(s.Keys, secrets.DomainMailMessage, ownerID, input.AccountID, storedID, "subject", storedSubject)
	if err != nil {
		return "", err
	}
	storedHeadersPlain, err := openField(s.Keys, secrets.DomainMailMessage, ownerID, input.AccountID, storedID, "headers", storedHeaders)
	if err != nil {
		return "", err
	}
	storedBodyPlain, err := openField(s.Keys, secrets.DomainMailMessage, ownerID, input.AccountID, storedID, "body", storedBody)
	if err != nil {
		return "", err
	}
	if storedFormat != input.BodyFormat ||
		!bytesEqual(storedSubjectPlain, []byte(input.Subject)) ||
		!bytesEqual(storedBodyPlain, []byte(input.Body)) ||
		!bytesEqual(storedHeadersPlain, headersJSON) {
		return "", ErrConflict
	}
	auditor := audit.Store{DB: tx}
	if err := auditor.Append(ctx, audit.Entry{
		OwnerID: ownerID, ActorKind: audit.ActorSystem, ActorID: "mail-sync",
		Action: "mail.message.import", SubjectKind: "mail_message", SubjectID: storedID,
		Detail: map[string]string{"account_id": input.AccountID, "status": StatusDuplicate},
	}, now); err != nil {
		return "", fmt.Errorf("audit mail import retry: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit mail import retry: %w", err)
	}
	return StatusDuplicate, nil
}

// GetMessage returns the fully decrypted message. Any decryption failure is
// an error; a zero-value message is never returned as success.
func (s Store) GetMessage(ctx context.Context, ownerID, messageID string) (Message, error) {
	row, err := s.messageRow(ctx, ownerID, messageID)
	if err != nil {
		return Message{}, err
	}
	return s.decryptMessage(ownerID, row)
}

type messageRow struct {
	ID              string
	AccountID       string
	Folder          string
	RemoteMessageID string
	ReceivedMs      int64
	SizeBytes       int64
	Unread          bool
	HasAttachments  bool
	BodyFormat      string
	Subject         []byte
	Headers         []byte
	Body            []byte
	AttachmentCount int
	CreatedMs       int64
}

func (s Store) messageRow(ctx context.Context, ownerID, messageID string) (messageRow, error) {
	row, err := scanMessageRow(singleRow{s.DB.QueryRowContext(ctx, messageSelect+" AND message_id = ?",
		ownerID, messageID)})
	if errors.Is(err, sql.ErrNoRows) {
		return messageRow{}, ErrNotFound
	}
	if err != nil {
		return messageRow{}, fmt.Errorf("read mail message: %w", err)
	}
	return row, nil
}

func (s Store) decryptMessage(ownerID string, row messageRow) (Message, error) {
	subjectPlain, err := openField(s.Keys, secrets.DomainMailMessage, ownerID, row.AccountID, row.ID, "subject", row.Subject)
	if err != nil {
		return Message{}, err
	}
	headersPlain, err := openField(s.Keys, secrets.DomainMailMessage, ownerID, row.AccountID, row.ID, "headers", row.Headers)
	if err != nil {
		return Message{}, err
	}
	bodyPlain, err := openField(s.Keys, secrets.DomainMailMessage, ownerID, row.AccountID, row.ID, "body", row.Body)
	if err != nil {
		return Message{}, err
	}
	var headers MessageHeaders
	if err := json.Unmarshal(headersPlain, &headers); err != nil {
		return Message{}, fmt.Errorf("%w: headers payload invalid", ErrCiphertext)
	}
	return Message{
		ID: row.ID, AccountID: row.AccountID, Folder: row.Folder,
		RemoteMessageID: row.RemoteMessageID, ReceivedAt: time.UnixMilli(row.ReceivedMs).UTC(),
		SizeBytes: row.SizeBytes, Unread: row.Unread, HasAttachments: row.HasAttachments,
		BodyFormat: row.BodyFormat, Subject: string(subjectPlain), Headers: headers,
		Body: string(bodyPlain), AttachmentCount: row.AttachmentCount,
		CreatedAt: time.UnixMilli(row.CreatedMs).UTC(),
	}, nil
}

// ListMessages returns one stable keyset page for the owner scope, newest
// first. An empty NextCursor marks the end.
func (s Store) ListMessages(ctx context.Context, ownerID string, opts ListOptions) (MessagePage, error) {
	limit := opts.Limit
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	atMs, id, err := cursor.Decode(opts.Cursor)
	if err != nil {
		return MessagePage{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(messageSelect)
	args := []any{ownerID}
	appendScope(&sqlQuery, &args, opts.AccountID, opts.Folder, atMs, id)
	sqlQuery.WriteString(" ORDER BY received_at_ms DESC, message_id DESC LIMIT ?")
	args = append(args, limit+1)
	rows, err := s.queryMessageRows(ctx, sqlQuery.String(), args)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list mail messages: %w", err)
	}
	page := MessagePage{Messages: make([]MessageSummary, 0, limit)}
	for i, row := range rows {
		if i == limit {
			// One extra row proves more pages follow; the page stays bounded.
			next, err := cursor.Encode(rows[i-1].ReceivedMs, rows[i-1].ID)
			if err != nil {
				return MessagePage{}, err
			}
			page.NextCursor = next
			break
		}
		message, err := s.decryptMessage(ownerID, row)
		if err != nil {
			return MessagePage{}, err
		}
		page.Messages = append(page.Messages, summaryFrom(message))
	}
	return page, nil
}

// SearchMessages decrypts records inside the authorized scope and matches the
// query against subject, sender and full body (RFC-0003 §6). No plaintext
// index is built or kept. The optional cursor continues a previous page at the
// same stable keyset position as the list endpoints; the scan bound applies
// from that position.
func (s Store) SearchMessages(ctx context.Context, ownerID string, opts SearchOptions) (MessagePage, error) {
	query := strings.TrimSpace(opts.Query)
	if !freeText(query, 1, 256) {
		return MessagePage{}, ErrInvalid
	}
	limit := opts.Limit
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	atMs, id, err := cursor.Decode(opts.Cursor)
	if err != nil {
		return MessagePage{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	needle := strings.ToLower(query)
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(messageSelect)
	args := []any{ownerID}
	appendScope(&sqlQuery, &args, opts.AccountID, opts.Folder, atMs, id)
	sqlQuery.WriteString(" ORDER BY received_at_ms DESC, message_id DESC LIMIT ?")
	args = append(args, maxSearchScan)
	rows, err := s.queryMessageRows(ctx, sqlQuery.String(), args)
	if err != nil {
		return MessagePage{}, fmt.Errorf("scan mail search scope: %w", err)
	}
	page := MessagePage{Messages: make([]MessageSummary, 0, limit)}
	for _, row := range rows {
		message, err := s.decryptMessage(ownerID, row)
		if err != nil {
			return MessagePage{}, err
		}
		if !strings.Contains(strings.ToLower(message.Subject), needle) &&
			!strings.Contains(strings.ToLower(message.Headers.From), needle) &&
			!strings.Contains(strings.ToLower(message.Body), needle) {
			continue
		}
		if len(page.Messages) == limit {
			// One extra matching row proves more matches follow; the page
			// stays bounded and the cursor pins the last returned match.
			last := page.Messages[limit-1]
			next, err := cursor.Encode(last.ReceivedAt.UnixMilli(), last.ID)
			if err != nil {
				return MessagePage{}, err
			}
			page.NextCursor = next
			break
		}
		page.Messages = append(page.Messages, summaryFrom(message))
	}
	return page, nil
}

const messageSelect = `SELECT message_id, account_id, folder, remote_message_id,
	received_at_ms, size_bytes, is_unread, has_attachments, body_format,
	subject_ciphertext, headers_ciphertext, body_ciphertext, attachment_count, created_at_ms
	FROM mail_messages WHERE owner_id = ?`

func appendScope(query *strings.Builder, args *[]any, accountID, folder string, atMs int64, id string) {
	if accountID != "" {
		query.WriteString(" AND account_id = ?")
		*args = append(*args, accountID)
	}
	if folder != "" {
		query.WriteString(" AND folder = ?")
		*args = append(*args, folder)
	}
	if id != "" {
		query.WriteString(" AND (received_at_ms, message_id) < (?, ?)")
		*args = append(*args, atMs, id)
	}
}

func (s Store) queryMessageRows(ctx context.Context, query string, args []any) ([]messageRow, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]messageRow, 0)
	for rows.Next() {
		row, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func scanMessageRow(scanner interface{ Scan(dest ...any) error }) (messageRow, error) {
	var row messageRow
	var unread, hasAttachments int
	err := scanner.Scan(&row.ID, &row.AccountID, &row.Folder, &row.RemoteMessageID,
		&row.ReceivedMs, &row.SizeBytes, &unread, &hasAttachments, &row.BodyFormat,
		&row.Subject, &row.Headers, &row.Body, &row.AttachmentCount, &row.CreatedMs)
	if err != nil {
		return messageRow{}, fmt.Errorf("scan mail message: %w", err)
	}
	row.Unread = unread == 1
	row.HasAttachments = hasAttachments == 1
	return row, nil
}

func summaryFrom(message Message) MessageSummary {
	return MessageSummary{
		ID: message.ID, AccountID: message.AccountID, Folder: message.Folder,
		ReceivedAt: message.ReceivedAt, Unread: message.Unread,
		HasAttachments: message.HasAttachments, AttachmentCount: message.AttachmentCount,
		Subject: message.Subject, From: message.Headers.From,
		Preview: truncate(message.Body, previewRunes),
	}
}

// AddAttachment registers attachment metadata without content.
func (s Store) AddAttachment(ctx context.Context, ownerID, messageID string, input AttachmentInput, now time.Time) (AttachmentMeta, error) {
	if !printable(input.Filename, 1, 255) || strings.ContainsAny(input.Filename, "/\\") ||
		!printable(input.MimeType, 1, 127) || input.SizeBytes < 0 || input.SizeBytes > maxAttachmentBytes {
		return AttachmentMeta{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return AttachmentMeta{}, fmt.Errorf("begin attachment creation: %w", err)
	}
	defer tx.Rollback()
	var accountID string
	err = tx.QueryRowContext(ctx, `SELECT account_id FROM mail_messages
		WHERE message_id = ? AND owner_id = ?`, messageID, ownerID).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return AttachmentMeta{}, ErrNotFound
	}
	if err != nil {
		return AttachmentMeta{}, fmt.Errorf("check mail message: %w", err)
	}
	attachmentID, err := randid.New()
	if err != nil {
		return AttachmentMeta{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO mail_attachments
		(attachment_id, message_id, owner_id, account_id, filename, mime_type, size_bytes,
		 download_state, created_at_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attachmentID, messageID, ownerID, accountID, input.Filename, input.MimeType,
		input.SizeBytes, DownloadRemoteOnly, now.UTC().UnixMilli()); err != nil {
		return AttachmentMeta{}, fmt.Errorf("create attachment: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return AttachmentMeta{}, fmt.Errorf("commit attachment creation: %w", err)
	}
	return AttachmentMeta{
		ID: attachmentID, MessageID: messageID, AccountID: accountID,
		Filename: input.Filename, MimeType: input.MimeType, SizeBytes: input.SizeBytes,
		DownloadState: DownloadRemoteOnly, CreatedAt: now.UTC(),
	}, nil
}

// StoreAttachmentContent seals and stores content after an explicit user
// triggered download. The content AAD binds owner, account and attachment.
func (s Store) StoreAttachmentContent(ctx context.Context, ownerID, attachmentID string, content []byte, now time.Time) error {
	if len(content) < 1 || len(content) > maxAttachmentBytes {
		return ErrInvalid
	}
	var accountID string
	err := s.DB.QueryRowContext(ctx, `SELECT account_id FROM mail_attachments
		WHERE attachment_id = ? AND owner_id = ?`, attachmentID, ownerID).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check attachment: %w", err)
	}
	contentCipher, err := sealField(s.Keys, secrets.DomainMailAttachment, ownerID, accountID, attachmentID, "content", content)
	if err != nil {
		return err
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE mail_attachments
		SET content_ciphertext = ?, download_state = ? WHERE attachment_id = ? AND owner_id = ?`,
		contentCipher, DownloadStored, attachmentID, ownerID)
	if err != nil {
		return fmt.Errorf("store attachment content: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return err
		}
		return ErrNotFound
	}
	return nil
}

// GetAttachment returns attachment metadata without content.
func (s Store) GetAttachment(ctx context.Context, ownerID, attachmentID string) (AttachmentMeta, error) {
	var meta AttachmentMeta
	var createdMs int64
	err := s.DB.QueryRowContext(ctx, `SELECT attachment_id, message_id, account_id, filename,
		mime_type, size_bytes, download_state, created_at_ms FROM mail_attachments
		WHERE attachment_id = ? AND owner_id = ?`, attachmentID, ownerID).
		Scan(&meta.ID, &meta.MessageID, &meta.AccountID, &meta.Filename, &meta.MimeType,
			&meta.SizeBytes, &meta.DownloadState, &createdMs)
	if errors.Is(err, sql.ErrNoRows) {
		return AttachmentMeta{}, ErrNotFound
	}
	if err != nil {
		return AttachmentMeta{}, fmt.Errorf("read attachment: %w", err)
	}
	meta.CreatedAt = time.UnixMilli(createdMs).UTC()
	return meta, nil
}

// ListAttachments returns the metadata of a message's attachments.
func (s Store) ListAttachments(ctx context.Context, ownerID, messageID string) ([]AttachmentMeta, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT attachment_id, message_id, account_id, filename,
		mime_type, size_bytes, download_state, created_at_ms FROM mail_attachments
		WHERE message_id = ? AND owner_id = ? ORDER BY created_at_ms, attachment_id`,
		messageID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()
	attachments := make([]AttachmentMeta, 0)
	for rows.Next() {
		var meta AttachmentMeta
		var createdMs int64
		if err := rows.Scan(&meta.ID, &meta.MessageID, &meta.AccountID, &meta.Filename,
			&meta.MimeType, &meta.SizeBytes, &meta.DownloadState, &createdMs); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		meta.CreatedAt = time.UnixMilli(createdMs).UTC()
		attachments = append(attachments, meta)
	}
	return attachments, rows.Err()
}

// GetAttachmentContent decrypts stored content; attachments that were never
// downloaded report ErrNotFound, decryption failures ErrCiphertext.
func (s Store) GetAttachmentContent(ctx context.Context, ownerID, attachmentID string) ([]byte, error) {
	var accountID string
	var stored []byte
	err := s.DB.QueryRowContext(ctx, `SELECT account_id, content_ciphertext
		FROM mail_attachments WHERE attachment_id = ? AND owner_id = ?`, attachmentID, ownerID).
		Scan(&accountID, &stored)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read attachment content: %w", err)
	}
	if stored == nil {
		return nil, ErrNotFound
	}
	return openField(s.Keys, secrets.DomainMailAttachment, ownerID, accountID, attachmentID, "content", stored)
}

func sealField(keys *secrets.Keyring, domain, ownerID, scopeID, recordID, field string, plaintext []byte) ([]byte, error) {
	aad, err := secrets.AAD(domain, ownerID, scopeID, recordID, field)
	if err != nil {
		return nil, err
	}
	data, err := keys.SealToBlob(aad, plaintext)
	if err != nil {
		return nil, fmt.Errorf("seal mail %s: %w", field, err)
	}
	return data, nil
}

func openField(keys *secrets.Keyring, domain, ownerID, scopeID, recordID, field string, stored []byte) ([]byte, error) {
	aad, err := secrets.AAD(domain, ownerID, scopeID, recordID, field)
	if err != nil {
		return nil, err
	}
	plaintext, err := keys.OpenFromBlob(aad, stored)
	if err != nil {
		return nil, fmt.Errorf("%w: mail %s: %v", ErrCiphertext, field, err)
	}
	return plaintext, nil
}

func validAddress(value string) bool {
	if !printable(value, 3, 254) {
		return false
	}
	local, domain, ok := strings.Cut(value, "@")
	return ok && local != "" && domain != "" && !strings.Contains(domain, "@")
}

func printable(value string, min, max int) bool {
	if !utf8.ValidString(value) || len(value) < min || len(value) > max {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func freeText(value string, min, max int) bool {
	if !utf8.ValidString(value) || len(value) < min || len(value) > max {
		return false
	}
	for _, r := range value {
		if r == 0 || unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func bytesEqual(a, b []byte) bool { return string(a) == string(b) }

func truncate(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}
