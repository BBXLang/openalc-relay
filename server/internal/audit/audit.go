// Package audit records bounded, credential-free metadata for security
// relevant state changes. Entries are written in the same transaction as the
// change they describe and never contain credentials, message bodies or phone
// numbers.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenALC/relay/server/internal/randid"
)

var (
	ErrNotFound = errors.New("resource not found")
	ErrInvalid  = errors.New("audit entry invalid")
)

// Actor kinds constrain who performed the action.
const (
	ActorOwnerSession = "owner_session"
	ActorDevice       = "device"
	ActorSystem       = "system"
)

// MaxDetailBytes bounds the serialized detail metadata.
const MaxDetailBytes = 2048

// DBTX is the subset of database handles the store needs; both *sql.DB and
// *sql.Tx satisfy it so entries join the caller's transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type Store struct{ DB DBTX }

type Entry struct {
	ID          string
	OwnerID     string
	ActorKind   string
	ActorID     string
	Action      string
	SubjectKind string
	SubjectID   string
	Detail      map[string]string
	CreatedAt   time.Time
}

// Append writes one audit entry inside the caller's transaction. Validation
// failures and oversized details fail closed instead of being truncated.
func (s Store) Append(ctx context.Context, entry Entry, now time.Time) error {
	if entry.OwnerID == "" || entry.SubjectID == "" {
		return ErrInvalid
	}
	switch entry.ActorKind {
	case ActorOwnerSession, ActorDevice, ActorSystem:
	default:
		return ErrInvalid
	}
	if !bounded(entry.ActorID, 1, 128) || !bounded(entry.Action, 1, 64) ||
		!bounded(entry.SubjectKind, 1, 32) || !bounded(entry.SubjectID, 1, 128) {
		return ErrInvalid
	}
	var detail any
	if len(entry.Detail) > 0 {
		for key, value := range entry.Detail {
			if key == "" || strings.ContainsRune(key, 0) || strings.ContainsRune(value, 0) {
				return ErrInvalid
			}
		}
		encoded, err := json.Marshal(entry.Detail)
		if err != nil {
			return fmt.Errorf("encode audit detail: %w", err)
		}
		if len(encoded) > MaxDetailBytes {
			return ErrInvalid
		}
		detail = string(encoded)
	}
	id, err := randid.New()
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO audit_entries
		(entry_id, owner_id, actor_kind, actor_id, action, subject_kind, subject_id, detail, created_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, entry.OwnerID, entry.ActorKind, entry.ActorID, entry.Action,
		entry.SubjectKind, entry.SubjectID, detail, now.UTC().UnixMilli())
	if err != nil {
		return fmt.Errorf("write audit entry: %w", err)
	}
	entry.ID = id
	return nil
}

// List returns the owner's most recent audit metadata, newest first.
func (s Store) List(ctx context.Context, ownerID string, limit int) ([]Entry, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT entry_id, actor_kind, actor_id, action,
		subject_kind, subject_id, detail, created_at_ms FROM audit_entries
		WHERE owner_id = ? ORDER BY created_at_ms DESC, entry_id DESC LIMIT ?`, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()
	entries := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		var detail sql.NullString
		var createdMs int64
		if err := rows.Scan(&entry.ID, &entry.ActorKind, &entry.ActorID, &entry.Action,
			&entry.SubjectKind, &entry.SubjectID, &detail, &createdMs); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		entry.OwnerID = ownerID
		entry.CreatedAt = time.UnixMilli(createdMs).UTC()
		if detail.Valid {
			var decoded map[string]string
			if err := json.Unmarshal([]byte(detail.String), &decoded); err != nil {
				return nil, fmt.Errorf("decode audit detail: %w", err)
			}
			entry.Detail = decoded
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func bounded(value string, min, max int) bool {
	return len(value) >= min && len(value) <= max && !strings.ContainsRune(value, 0)
}
