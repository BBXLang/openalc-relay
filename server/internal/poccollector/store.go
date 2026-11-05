// Package poccollector is a synthetic-only, loopback test harness, not the Relay business API.
package poccollector

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"

	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

const Fixture = "openalc-relay-synthetic-v1"
const MaxBatchSize = 25

//go:embed migrations/*.sql
var migrationFiles embed.FS

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Event struct {
	EventID        string `json:"event_id"`
	FixtureID      string `json:"fixture_id"`
	Source         string `json:"source"`
	ReceivedAtMS   int64  `json:"received_at_ms"`
	SubscriptionID *int32 `json:"subscription_id"`
}

type Batch struct {
	Version        int     `json:"version"`
	Fixture        string  `json:"fixture"`
	InstallationID string  `json:"installation_id"`
	Events         []Event `json:"events"`
}

type Acknowledgement struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
}

type Receipt struct {
	Version          int               `json:"version"`
	InstallationID   string            `json:"installation_id"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
}

type Store struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*Store, error) {
	source, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	db, err := sqlite.OpenWithMigrations(ctx, path, source)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func validBatch(batch Batch) bool {
	if batch.Version != 1 || batch.Fixture != Fixture || !uuid.MatchString(batch.InstallationID) || len(batch.Events) < 1 || len(batch.Events) > MaxBatchSize {
		return false
	}
	seen := make(map[string]bool)
	for _, event := range batch.Events {
		if !uuid.MatchString(event.EventID) || seen[event.EventID] {
			return false
		}
		seen[event.EventID] = true
	}
	return true
}

func validEvent(event Event) bool {
	return event.FixtureID == event.EventID &&
		(event.Source == "injected" || event.Source == "sms_broadcast") &&
		event.ReceivedAtMS > 0 && event.ReceivedAtMS < 4102444800000 &&
		(event.SubscriptionID == nil || *event.SubscriptionID >= 0)
}

// Persist commits accepted events before returning a receipt. Rejected events remain unconfirmed.
func (s *Store) Persist(ctx context.Context, batch Batch) (Receipt, error) {
	if !validBatch(batch) {
		return Receipt{}, fmt.Errorf("invalid synthetic batch")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Receipt{}, err
	}
	defer tx.Rollback()
	receipt := Receipt{Version: 1, InstallationID: batch.InstallationID, Acknowledgements: make([]Acknowledgement, 0, len(batch.Events))}
	for _, event := range batch.Events {
		ack := Acknowledgement{EventID: event.EventID}
		if !validEvent(event) {
			ack.Status, ack.Code = "rejected", "invalid_event"
			receipt.Acknowledgements = append(receipt.Acknowledgements, ack)
			continue
		}
		payload, err := json.Marshal(event)
		if err != nil {
			return Receipt{}, err
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
		result, err := tx.ExecContext(ctx, `INSERT INTO synthetic_events
			(installation_id, event_id, fixture_id, source, received_at_ms, subscription_id, payload_checksum)
			VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (installation_id, event_id) DO NOTHING`,
			batch.InstallationID, event.EventID, event.FixtureID, event.Source, event.ReceivedAtMS, event.SubscriptionID, checksum)
		if err != nil {
			return Receipt{}, err
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return Receipt{}, err
		}
		ack.Status = "persisted"
		if inserted == 0 {
			var existing string
			if err := tx.QueryRowContext(ctx, "SELECT payload_checksum FROM synthetic_events WHERE installation_id = ? AND event_id = ?", batch.InstallationID, event.EventID).Scan(&existing); err != nil {
				return Receipt{}, err
			}
			ack.Status = "duplicate"
			if existing != checksum {
				ack.Status, ack.Code = "rejected", "event_id_conflict"
			}
		}
		receipt.Acknowledgements = append(receipt.Acknowledgements, ack)
	}
	if err := tx.Commit(); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func (s *Store) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM synthetic_events").Scan(&count)
	return count, err
}

type EventPage struct {
	InstallationID string  `json:"installation_id"`
	Events         []Event `json:"events"`
	Total          int     `json:"total"`
	NextAfter      *string `json:"next_after"`
}

// List reads one installation using stable UUID ordering. Refresh to see inserts before a cursor.
func (s *Store) List(ctx context.Context, installationID, after string, limit int) (EventPage, error) {
	page := EventPage{InstallationID: installationID, Events: []Event{}}
	if !uuid.MatchString(installationID) || (after != "" && !uuid.MatchString(after)) || limit < 1 || limit > 50 {
		return page, fmt.Errorf("invalid synthetic query")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM synthetic_events WHERE installation_id = ?", installationID).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT event_id, fixture_id, source, received_at_ms, subscription_id
		FROM synthetic_events WHERE installation_id = ? AND event_id > ? ORDER BY event_id LIMIT ?`, installationID, after, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.EventID, &event.FixtureID, &event.Source, &event.ReceivedAtMS, &event.SubscriptionID); err != nil {
			return page, err
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if len(page.Events) > limit {
		page.Events = page.Events[:limit]
		last := page.Events[limit-1].EventID
		page.NextAfter = &last
	}
	if err := rows.Close(); err != nil {
		return page, err
	}
	return page, tx.Commit()
}
