// Package smsstore implements encrypted storage for SMS records and the
// single-send command state machine. Records are scoped to owner and device,
// imports deduplicate on (device_id, remote_event_id), commands enforce the
// queued/claimed/system_sent/resolved transitions and once a send may have
// been invoked the command can never be re-queued automatically.
package smsstore

import (
	"context"
	"database/sql"
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
	ErrNotFound    = errors.New("resource not found")
	ErrConflict    = errors.New("resource already used")
	ErrDenied      = errors.New("resource denied")
	ErrCiphertext  = errors.New("encrypted payload rejected")
	ErrInvalid     = errors.New("request invalid")
	ErrRateLimited = errors.New("send rate limit exceeded")
)

// Import result statuses.
const (
	StatusAccepted  = "accepted"
	StatusDuplicate = "duplicate"
)

// Directions and command statuses mirrored by the schema CHECK.
const (
	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"

	StatusQueued          = "queued"
	StatusClaimed         = "claimed"
	StatusSystemSent      = "system_sent"
	StatusDeliveryReceipt = "delivery_receipt"
	StatusFailed          = "failed"
	StatusUnknown         = "unknown"
)

// MaxQueuedCommandsPerOwner bounds the number of simultaneously queued send
// commands; request level rate limiting is the calling layer's concern.
const MaxQueuedCommandsPerOwner = 10

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxSearchScan   = 500
)

type Store struct {
	DB   *sql.DB
	Keys *secrets.Keyring
}

// Message is the decrypted view of one stored SMS record.
type Message struct {
	ID         string    `json:"sms_id"`
	DeviceID   string    `json:"device_id"`
	Direction  string    `json:"direction"`
	Sender     string    `json:"sender"`
	Recipient  string    `json:"recipient"`
	Body       string    `json:"body"`
	DeviceAt   time.Time `json:"device_at"`
	ReceivedAt time.Time `json:"received_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type MessageInput struct {
	DeviceID      string
	Direction     string
	RemoteEventID string
	Sender        string
	Recipient     string
	Body          string
	DeviceAt      time.Time
}

type ListOptions struct {
	DeviceID string
	Cursor   string
	Limit    int
}

type MessagePage struct {
	Messages   []Message `json:"messages"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

type SearchOptions struct {
	DeviceID string
	Query    string
	Limit    int
}

// Command is the decrypted view of one single-send command. The state machine
// invariants are documented in RFC-0003 §3.
type Command struct {
	ID             string     `json:"command_id"`
	DeviceID       string     `json:"device_id"`
	IdempotencyKey string     `json:"idempotency_key"`
	Recipient      string     `json:"recipient"`
	Body           string     `json:"body"`
	Status         string     `json:"status"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
	SystemSentAt   *time.Time `json:"system_sent_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type CommandInput struct {
	DeviceID       string
	IdempotencyKey string
	Recipient      string
	Body           string
}

// InsertMessage stores one SMS record in a single transaction. A repeated
// (device_id, remote_event_id) with identical decrypted content returns
// StatusDuplicate; the same key with different content fails with ErrConflict.
func (s Store) InsertMessage(ctx context.Context, ownerID string, input MessageInput, now time.Time) (string, error) {
	if input.Direction != DirectionInbound && input.Direction != DirectionOutbound ||
		!printable(input.RemoteEventID, 1, 128) || !printable(input.Sender, 1, 64) ||
		!printable(input.Recipient, 1, 64) || !freeText(input.Body, 1, 1600) ||
		input.DeviceAt.IsZero() {
		return "", ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin sms import: %w", err)
	}
	defer tx.Rollback()
	if err := checkDevice(ctx, tx, ownerID, input.DeviceID); err != nil {
		return "", err
	}
	smsID, err := randid.New()
	if err != nil {
		return "", err
	}
	sender, err := sealField(s.Keys, ownerID, input.DeviceID, smsID, "sender", []byte(input.Sender))
	if err != nil {
		return "", err
	}
	recipient, err := sealField(s.Keys, ownerID, input.DeviceID, smsID, "recipient", []byte(input.Recipient))
	if err != nil {
		return "", err
	}
	body, err := sealField(s.Keys, ownerID, input.DeviceID, smsID, "body", []byte(input.Body))
	if err != nil {
		return "", err
	}
	inserted, err := tx.ExecContext(ctx, `INSERT INTO sms_messages
		(sms_id, owner_id, device_id, direction, remote_event_id, sender_ciphertext,
		 recipient_ciphertext, body_ciphertext, device_at_ms, received_at_ms, created_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id, remote_event_id) DO NOTHING`,
		smsID, ownerID, input.DeviceID, input.Direction, input.RemoteEventID, sender,
		recipient, body, input.DeviceAt.UTC().UnixMilli(), now.UTC().UnixMilli(), now.UTC().UnixMilli())
	if err != nil {
		return "", fmt.Errorf("store sms message: %w", err)
	}
	count, err := inserted.RowsAffected()
	if err != nil {
		return "", err
	}
	status := StatusAccepted
	subjectID := smsID
	if count != 1 {
		// Duplicate key: fail closed unless the decrypted content matches
		// exactly; the same key with different content is a conflict.
		var storedID, storedDirection string
		var storedSender, storedRecipient, storedBody []byte
		err = tx.QueryRowContext(ctx, `SELECT sms_id, direction, sender_ciphertext,
			recipient_ciphertext, body_ciphertext FROM sms_messages
			WHERE device_id = ? AND remote_event_id = ?`, input.DeviceID, input.RemoteEventID).
			Scan(&storedID, &storedDirection, &storedSender, &storedRecipient, &storedBody)
		if err != nil {
			return "", fmt.Errorf("read stored sms message: %w", err)
		}
		storedSenderPlain, err := openField(s.Keys, ownerID, input.DeviceID, storedID, "sender", storedSender)
		if err != nil {
			return "", err
		}
		storedRecipientPlain, err := openField(s.Keys, ownerID, input.DeviceID, storedID, "recipient", storedRecipient)
		if err != nil {
			return "", err
		}
		storedBodyPlain, err := openField(s.Keys, ownerID, input.DeviceID, storedID, "body", storedBody)
		if err != nil {
			return "", err
		}
		if storedDirection != input.Direction ||
			string(storedSenderPlain) != input.Sender ||
			string(storedRecipientPlain) != input.Recipient ||
			string(storedBodyPlain) != input.Body {
			return "", ErrConflict
		}
		status = StatusDuplicate
		subjectID = storedID
	}
	auditor := audit.Store{DB: tx}
	if err := auditor.Append(ctx, audit.Entry{
		OwnerID: ownerID, ActorKind: audit.ActorDevice, ActorID: input.DeviceID,
		Action: "sms.message.receive", SubjectKind: "sms_message", SubjectID: subjectID,
		Detail: map[string]string{"status": status, "direction": input.Direction},
	}, now); err != nil {
		return "", fmt.Errorf("audit sms import: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit sms import: %w", err)
	}
	return status, nil
}

// GetMessage returns the decrypted message; decryption failures are errors.
func (s Store) GetMessage(ctx context.Context, ownerID, smsID string) (Message, error) {
	row, err := s.messageRow(ctx, ownerID, smsID)
	if err != nil {
		return Message{}, err
	}
	return s.decryptMessage(ownerID, row)
}

// ListMessages returns one stable keyset page for the owner scope, newest
// first, optionally filtered by device.
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
	if opts.DeviceID != "" {
		sqlQuery.WriteString(" AND device_id = ?")
		args = append(args, opts.DeviceID)
	}
	if id != "" {
		sqlQuery.WriteString(" AND (received_at_ms, sms_id) < (?, ?)")
		args = append(args, atMs, id)
	}
	sqlQuery.WriteString(" ORDER BY received_at_ms DESC, sms_id DESC LIMIT ?")
	args = append(args, limit+1)
	rows, err := s.queryMessageRows(ctx, sqlQuery.String(), args)
	if err != nil {
		return MessagePage{}, fmt.Errorf("list sms messages: %w", err)
	}
	page := MessagePage{Messages: make([]Message, 0, limit)}
	for i, row := range rows {
		if i == limit {
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
		page.Messages = append(page.Messages, message)
	}
	return page, nil
}

// Search decrypts records inside the authorized scope and matches the query
// against sender, recipient and body. No plaintext index is built or kept.
func (s Store) Search(ctx context.Context, ownerID string, opts SearchOptions) ([]Message, error) {
	query := strings.TrimSpace(opts.Query)
	if !freeText(query, 1, 256) {
		return nil, ErrInvalid
	}
	limit := opts.Limit
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	needle := strings.ToLower(query)
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(messageSelect)
	args := []any{ownerID}
	if opts.DeviceID != "" {
		sqlQuery.WriteString(" AND device_id = ?")
		args = append(args, opts.DeviceID)
	}
	sqlQuery.WriteString(" ORDER BY received_at_ms DESC, sms_id DESC LIMIT ?")
	args = append(args, maxSearchScan)
	rows, err := s.queryMessageRows(ctx, sqlQuery.String(), args)
	if err != nil {
		return nil, fmt.Errorf("scan sms search scope: %w", err)
	}
	matches := make([]Message, 0, limit)
	for _, row := range rows {
		message, err := s.decryptMessage(ownerID, row)
		if err != nil {
			return nil, err
		}
		if strings.Contains(strings.ToLower(message.Sender), needle) ||
			strings.Contains(strings.ToLower(message.Recipient), needle) ||
			strings.Contains(strings.ToLower(message.Body), needle) {
			matches = append(matches, message)
			if len(matches) == limit {
				break
			}
		}
	}
	return matches, nil
}

// CreateCommand enqueues one single-send command. The idempotency key is
// unique per owner: identical retries return the existing command with
// StatusDuplicate, different content under the same key fails with ErrConflict,
// and no path ever re-enqueues a command that left the queued state.
func (s Store) CreateCommand(ctx context.Context, ownerID string, input CommandInput, now time.Time) (Command, string, error) {
	if !printable(input.IdempotencyKey, 16, 128) || !printable(input.Recipient, 1, 64) ||
		!freeText(input.Body, 1, 1600) {
		return Command{}, "", ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Command{}, "", fmt.Errorf("begin send command: %w", err)
	}
	defer tx.Rollback()
	if err := checkDevice(ctx, tx, ownerID, input.DeviceID); err != nil {
		return Command{}, "", err
	}
	var queued int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sms_send_commands
		WHERE owner_id = ? AND status = ?`, ownerID, StatusQueued).Scan(&queued); err != nil {
		return Command{}, "", fmt.Errorf("count queued commands: %w", err)
	}
	if queued >= MaxQueuedCommandsPerOwner {
		return Command{}, "", ErrRateLimited
	}
	commandID, err := randid.New()
	if err != nil {
		return Command{}, "", err
	}
	recipient, err := sealCommandField(s.Keys, ownerID, input.DeviceID, commandID, "recipient", []byte(input.Recipient))
	if err != nil {
		return Command{}, "", err
	}
	body, err := sealCommandField(s.Keys, ownerID, input.DeviceID, commandID, "body", []byte(input.Body))
	if err != nil {
		return Command{}, "", err
	}
	inserted, err := tx.ExecContext(ctx, `INSERT INTO sms_send_commands
		(command_id, owner_id, device_id, idempotency_key, recipient_ciphertext,
		 body_ciphertext, status, created_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(owner_id, idempotency_key) DO NOTHING`,
		commandID, ownerID, input.DeviceID, input.IdempotencyKey, recipient, body,
		StatusQueued, now.UTC().UnixMilli())
	if err != nil {
		return Command{}, "", fmt.Errorf("store send command: %w", err)
	}
	count, err := inserted.RowsAffected()
	if err != nil {
		return Command{}, "", err
	}
	if count == 1 {
		auditor := audit.Store{DB: tx}
		if err := auditor.Append(ctx, audit.Entry{
			OwnerID: ownerID, ActorKind: audit.ActorOwnerSession, ActorID: ownerID,
			Action: "sms.command.create", SubjectKind: "sms_command", SubjectID: commandID,
			Detail: map[string]string{"device_id": input.DeviceID, "status": StatusQueued},
		}, now); err != nil {
			return Command{}, "", fmt.Errorf("audit send command: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return Command{}, "", fmt.Errorf("commit send command: %w", err)
		}
		command, err := s.GetCommand(ctx, ownerID, commandID)
		if err != nil {
			return Command{}, "", err
		}
		return command, StatusAccepted, nil
	}
	// Duplicate idempotency key: fail closed unless device and content match.
	var storedID, storedDevice, storedStatus string
	var storedRecipient, storedBody []byte
	err = tx.QueryRowContext(ctx, `SELECT command_id, device_id, status,
		recipient_ciphertext, body_ciphertext FROM sms_send_commands
		WHERE owner_id = ? AND idempotency_key = ?`, ownerID, input.IdempotencyKey).
		Scan(&storedID, &storedDevice, &storedStatus, &storedRecipient, &storedBody)
	if err != nil {
		return Command{}, "", fmt.Errorf("read stored send command: %w", err)
	}
	storedRecipientPlain, err := openCommandField(s.Keys, ownerID, storedDevice, storedID, "recipient", storedRecipient)
	if err != nil {
		return Command{}, "", err
	}
	storedBodyPlain, err := openCommandField(s.Keys, ownerID, storedDevice, storedID, "body", storedBody)
	if err != nil {
		return Command{}, "", err
	}
	if storedDevice != input.DeviceID ||
		string(storedRecipientPlain) != input.Recipient || string(storedBodyPlain) != input.Body {
		return Command{}, "", ErrConflict
	}
	command, err := s.commandFromStored(ownerID, commandRow{
		ID: storedID, DeviceID: storedDevice, Status: storedStatus,
		Recipient: storedRecipient, Body: storedBody,
	}, true)
	if err != nil {
		return Command{}, "", err
	}
	return command, StatusDuplicate, nil
}

// GetCommand returns the decrypted command for the owner.
func (s Store) GetCommand(ctx context.Context, ownerID, commandID string) (Command, error) {
	row, err := s.commandRow(ctx, ownerID, commandID)
	if err != nil {
		return Command{}, err
	}
	return s.commandFromStored(ownerID, row, true)
}

// PendingForDevice returns queued commands for the given device, oldest first.
// A device can only ever see commands bound to its own device_id.
func (s Store) PendingForDevice(ctx context.Context, deviceID string, limit int) ([]Command, error) {
	if limit < 1 || limit > maxPageSize {
		limit = defaultPageSize
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT owner_id, command_id, device_id, idempotency_key, status,
		claimed_at_ms, system_sent_at_ms, resolved_at_ms, created_at_ms,
		recipient_ciphertext, body_ciphertext FROM sms_send_commands
		WHERE device_id = ? AND status = ? ORDER BY created_at_ms, command_id LIMIT ?`,
		deviceID, StatusQueued, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending commands: %w", err)
	}
	defer rows.Close()
	commands := make([]Command, 0)
	for rows.Next() {
		var row commandRow
		var ownerID string
		var claimedMs, sentMs, resolvedMs sql.NullInt64
		if err := rows.Scan(&ownerID, &row.ID, &row.DeviceID, &row.IdempotencyKey, &row.Status,
			&claimedMs, &sentMs, &resolvedMs, &row.CreatedMs, &row.Recipient, &row.Body); err != nil {
			return nil, fmt.Errorf("scan pending command: %w", err)
		}
		row.ClaimedMs, row.SystemSentMs, row.ResolvedMs = claimedMs, sentMs, resolvedMs
		command, err := s.commandFromStored(ownerID, row, true)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}
	return commands, rows.Err()
}

// Claim moves a queued command to claimed. Re-claiming an already claimed
// command is a no-op; any other source state is rejected.
func (s Store) Claim(ctx context.Context, deviceID, commandID string, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin command claim: %w", err)
	}
	defer tx.Rollback()
	var ownerID, status string
	err = tx.QueryRowContext(ctx, `SELECT owner_id, status FROM sms_send_commands
		WHERE command_id = ? AND device_id = ?`, commandID, deviceID).Scan(&ownerID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read command claim state: %w", err)
	}
	if status != StatusQueued && status != StatusClaimed {
		return ErrConflict
	}
	if status == StatusQueued {
		if _, err := tx.ExecContext(ctx, `UPDATE sms_send_commands SET status = ?,
			claimed_at_ms = ? WHERE command_id = ? AND device_id = ?`,
			StatusClaimed, now.UTC().UnixMilli(), commandID, deviceID); err != nil {
			return fmt.Errorf("claim command: %w", err)
		}
		auditor := audit.Store{DB: tx}
		if err := auditor.Append(ctx, audit.Entry{
			OwnerID: ownerID, ActorKind: audit.ActorDevice, ActorID: deviceID,
			Action: "sms.command.claim", SubjectKind: "sms_command", SubjectID: commandID,
			Detail: map[string]string{"status": StatusClaimed},
		}, now); err != nil {
			return fmt.Errorf("audit command claim: %w", err)
		}
	}
	return tx.Commit()
}

// MarkSystemSent records that the device invoked the system send API.
func (s Store) MarkSystemSent(ctx context.Context, deviceID, commandID string, now time.Time) error {
	return s.deviceTransition(ctx, deviceID, commandID, []string{StatusClaimed}, StatusSystemSent,
		"system_sent_at_ms", "sms.command.system_sent", now)
}

// MarkDeliveryReceipt records a carrier delivery receipt.
func (s Store) MarkDeliveryReceipt(ctx context.Context, deviceID, commandID string, now time.Time) error {
	return s.deviceTransition(ctx, deviceID, commandID, []string{StatusSystemSent}, StatusDeliveryReceipt,
		"resolved_at_ms", "sms.command.delivery_receipt", now)
}

// MarkFailed records an explicitly reported failure before or after the send
// call. It is never chosen automatically.
func (s Store) MarkFailed(ctx context.Context, deviceID, commandID string, now time.Time) error {
	return s.deviceTransition(ctx, deviceID, commandID, []string{StatusClaimed, StatusSystemSent}, StatusFailed,
		"resolved_at_ms", "sms.command.failed", now)
}

// MarkUnknown records that the result cannot be determined. The state is
// terminal: the send must never be retried automatically.
func (s Store) MarkUnknown(ctx context.Context, deviceID, commandID string, now time.Time) error {
	return s.deviceTransition(ctx, deviceID, commandID, []string{StatusClaimed, StatusSystemSent}, StatusUnknown,
		"resolved_at_ms", "sms.command.unknown", now)
}

func (s Store) deviceTransition(ctx context.Context, deviceID, commandID string, fromStates []string, to, stampColumn, action string, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin command transition: %w", err)
	}
	defer tx.Rollback()
	var ownerID string
	err = tx.QueryRowContext(ctx, `SELECT owner_id FROM sms_send_commands
		WHERE command_id = ? AND device_id = ?`, commandID, deviceID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read command transition state: %w", err)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(fromStates)), ", ")
	args := []any{to, now.UTC().UnixMilli(), commandID, deviceID}
	for _, state := range fromStates {
		args = append(args, state)
	}
	result, err := tx.ExecContext(ctx, `UPDATE sms_send_commands SET status = ?,
		`+stampColumn+` = ? WHERE command_id = ? AND device_id = ? AND status IN (`+placeholders+`)`,
		args...)
	if err != nil {
		return fmt.Errorf("apply command transition: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	auditor := audit.Store{DB: tx}
	if err := auditor.Append(ctx, audit.Entry{
		OwnerID: ownerID, ActorKind: audit.ActorDevice, ActorID: deviceID,
		Action: action, SubjectKind: "sms_command", SubjectID: commandID,
		Detail: map[string]string{"status": to},
	}, now); err != nil {
		return fmt.Errorf("audit command transition: %w", err)
	}
	return tx.Commit()
}

const messageSelect = `SELECT sms_id, device_id, direction, remote_event_id,
	received_at_ms, device_at_ms, created_at_ms, sender_ciphertext,
	recipient_ciphertext, body_ciphertext FROM sms_messages WHERE owner_id = ?`

type messageRow struct {
	ID          string
	DeviceID    string
	Direction   string
	RemoteEvent string
	ReceivedMs  int64
	DeviceMs    int64
	CreatedMs   int64
	Sender      []byte
	Recipient   []byte
	Body        []byte
}

func (s Store) queryMessageRows(ctx context.Context, query string, args []any) ([]messageRow, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]messageRow, 0)
	for rows.Next() {
		var row messageRow
		if err := rows.Scan(&row.ID, &row.DeviceID, &row.Direction, &row.RemoteEvent,
			&row.ReceivedMs, &row.DeviceMs, &row.CreatedMs, &row.Sender, &row.Recipient,
			&row.Body); err != nil {
			return nil, fmt.Errorf("scan sms message: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s Store) messageRow(ctx context.Context, ownerID, smsID string) (messageRow, error) {
	var row messageRow
	err := s.DB.QueryRowContext(ctx, messageSelect+" AND sms_id = ?", ownerID, smsID).
		Scan(&row.ID, &row.DeviceID, &row.Direction, &row.RemoteEvent, &row.ReceivedMs,
			&row.DeviceMs, &row.CreatedMs, &row.Sender, &row.Recipient, &row.Body)
	if errors.Is(err, sql.ErrNoRows) {
		return messageRow{}, ErrNotFound
	}
	if err != nil {
		return messageRow{}, fmt.Errorf("read sms message: %w", err)
	}
	return row, nil
}

func (s Store) decryptMessage(ownerID string, row messageRow) (Message, error) {
	sender, err := openField(s.Keys, ownerID, row.DeviceID, row.ID, "sender", row.Sender)
	if err != nil {
		return Message{}, err
	}
	recipient, err := openField(s.Keys, ownerID, row.DeviceID, row.ID, "recipient", row.Recipient)
	if err != nil {
		return Message{}, err
	}
	body, err := openField(s.Keys, ownerID, row.DeviceID, row.ID, "body", row.Body)
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: row.ID, DeviceID: row.DeviceID, Direction: row.Direction,
		Sender: string(sender), Recipient: string(recipient), Body: string(body),
		DeviceAt:   time.UnixMilli(row.DeviceMs).UTC(),
		ReceivedAt: time.UnixMilli(row.ReceivedMs).UTC(),
		CreatedAt:  time.UnixMilli(row.CreatedMs).UTC(),
	}, nil
}

type commandRow struct {
	ID             string
	OwnerID        string
	DeviceID       string
	IdempotencyKey string
	Status         string
	ClaimedMs      sql.NullInt64
	SystemSentMs   sql.NullInt64
	ResolvedMs     sql.NullInt64
	CreatedMs      int64
	Recipient      []byte
	Body           []byte
}

func (s Store) commandRow(ctx context.Context, ownerID, commandID string) (commandRow, error) {
	var row commandRow
	row.OwnerID = ownerID
	err := s.DB.QueryRowContext(ctx, `SELECT command_id, device_id, idempotency_key, status,
		claimed_at_ms, system_sent_at_ms, resolved_at_ms, created_at_ms,
		recipient_ciphertext, body_ciphertext FROM sms_send_commands
		WHERE owner_id = ? AND command_id = ?`, ownerID, commandID).
		Scan(&row.ID, &row.DeviceID, &row.IdempotencyKey, &row.Status, &row.ClaimedMs,
			&row.SystemSentMs, &row.ResolvedMs, &row.CreatedMs, &row.Recipient, &row.Body)
	if errors.Is(err, sql.ErrNoRows) {
		return commandRow{}, ErrNotFound
	}
	if err != nil {
		return commandRow{}, fmt.Errorf("read send command: %w", err)
	}
	return row, nil
}

func (s Store) commandFromStored(ownerID string, row commandRow, decrypt bool) (Command, error) {
	command := Command{
		ID: row.ID, DeviceID: row.DeviceID, IdempotencyKey: row.IdempotencyKey,
		Status: row.Status, CreatedAt: time.UnixMilli(row.CreatedMs).UTC(),
	}
	if row.ClaimedMs.Valid {
		at := time.UnixMilli(row.ClaimedMs.Int64).UTC()
		command.ClaimedAt = &at
	}
	if row.SystemSentMs.Valid {
		at := time.UnixMilli(row.SystemSentMs.Int64).UTC()
		command.SystemSentAt = &at
	}
	if row.ResolvedMs.Valid {
		at := time.UnixMilli(row.ResolvedMs.Int64).UTC()
		command.ResolvedAt = &at
	}
	if !decrypt {
		return command, nil
	}
	recipient, err := openCommandField(s.Keys, ownerID, row.DeviceID, row.ID, "recipient", row.Recipient)
	if err != nil {
		return Command{}, err
	}
	body, err := openCommandField(s.Keys, ownerID, row.DeviceID, row.ID, "body", row.Body)
	if err != nil {
		return Command{}, err
	}
	command.Recipient = string(recipient)
	command.Body = string(body)
	return command, nil
}

// checkDevice verifies the device belongs to the owner and is not revoked.
func checkDevice(ctx context.Context, tx *sql.Tx, ownerID, deviceID string) error {
	var revoked bool
	err := tx.QueryRowContext(ctx, `SELECT revoked_at_ms IS NOT NULL FROM devices
		WHERE device_id = ? AND owner_id = ?`, deviceID, ownerID).Scan(&revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check device: %w", err)
	}
	if revoked {
		return ErrDenied
	}
	return nil
}

func sealField(keys *secrets.Keyring, ownerID, scopeID, recordID, field string, plaintext []byte) ([]byte, error) {
	aad, err := secrets.AAD(secrets.DomainSMSMessage, ownerID, scopeID, recordID, field)
	if err != nil {
		return nil, err
	}
	data, err := keys.SealToBlob(aad, plaintext)
	if err != nil {
		return nil, fmt.Errorf("seal sms %s: %w", field, err)
	}
	return data, nil
}

// sealCommandField uses the sms_command domain so command payloads cannot be
// replayed as message payloads or vice versa.
func sealCommandField(keys *secrets.Keyring, ownerID, scopeID, recordID, field string, plaintext []byte) ([]byte, error) {
	aad, err := secrets.AAD(secrets.DomainSMSCommand, ownerID, scopeID, recordID, field)
	if err != nil {
		return nil, err
	}
	data, err := keys.SealToBlob(aad, plaintext)
	if err != nil {
		return nil, fmt.Errorf("seal command %s: %w", field, err)
	}
	return data, nil
}

func openField(keys *secrets.Keyring, ownerID, scopeID, recordID, field string, stored []byte) ([]byte, error) {
	aad, err := secrets.AAD(secrets.DomainSMSMessage, ownerID, scopeID, recordID, field)
	if err != nil {
		return nil, err
	}
	plaintext, err := keys.OpenFromBlob(aad, stored)
	if err != nil {
		return nil, fmt.Errorf("%w: sms %s: %v", ErrCiphertext, field, err)
	}
	return plaintext, nil
}

// openCommandField mirrors sealCommandField.
func openCommandField(keys *secrets.Keyring, ownerID, scopeID, recordID, field string, stored []byte) ([]byte, error) {
	aad, err := secrets.AAD(secrets.DomainSMSCommand, ownerID, scopeID, recordID, field)
	if err != nil {
		return nil, err
	}
	plaintext, err := keys.OpenFromBlob(aad, stored)
	if err != nil {
		return nil, fmt.Errorf("%w: command %s: %v", ErrCiphertext, field, err)
	}
	return plaintext, nil
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
