// Package synthetic implements the deliberately limited device exchange used to
// validate pairing and durable two-way metadata sync without communication data.
package synthetic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const PairingLifetime = 5 * time.Minute

var (
	ErrNotFound = errors.New("resource not found")
	ErrConflict = errors.New("resource already used")
	ErrExpired  = errors.New("pairing expired")
	ErrDenied   = errors.New("credential denied")
	ErrCode     = errors.New("comparison code incorrect")
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Store struct{ DB *sql.DB }

type Pairing struct {
	ID        string    `json:"pairing_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Request   *Request  `json:"request,omitempty"`
}

type Request struct {
	ID             string `json:"request_id"`
	DeviceLabel    string `json:"device_label"`
	ComparisonCode string `json:"comparison_code"`
	Status         string `json:"status"`
	ClaimToken     string `json:"claim_token,omitempty"`
}

type Device struct {
	ID          string `json:"device_id"`
	Label       string `json:"device_label"`
	Revoked     bool   `json:"revoked"`
	DeviceToken string `json:"device_token,omitempty"`
}

type Event struct {
	DeviceID      string    `json:"device_id"`
	ClientEventID string    `json:"client_event_id"`
	ReceivedAt    time.Time `json:"received_at"`
}

type Ping struct {
	ID             string     `json:"ping_id"`
	DeviceID       string     `json:"device_id"`
	CreatedAt      time.Time  `json:"created_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
}

type SyncResult struct {
	EventStatus string `json:"event_status,omitempty"`
	Pings       []Ping `json:"pings"`
}

func (s Store) CreatePairing(ctx context.Context, ownerID string, now time.Time) (Pairing, error) {
	id, err := randomID()
	if err != nil {
		return Pairing{}, err
	}
	expires := now.UTC().Add(PairingLifetime)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO device_pairings
		(pairing_id, owner_id, created_at_ms, expires_at_ms) VALUES (?, ?, ?, ?)`,
		id, ownerID, now.UTC().UnixMilli(), expires.UnixMilli())
	if err != nil {
		return Pairing{}, fmt.Errorf("create pairing: %w", err)
	}
	return Pairing{ID: id, ExpiresAt: expires}, nil
}

func (s Store) GetPairing(ctx context.Context, ownerID, pairingID string) (Pairing, error) {
	var expiresMs int64
	var requestID, label, code sql.NullString
	var approvedMs, exchangedMs sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT p.expires_at_ms, r.request_id, r.device_label,
		r.comparison_code, r.approved_at_ms, r.exchanged_at_ms
		FROM device_pairings p LEFT JOIN pairing_requests r ON r.pairing_id = p.pairing_id
		WHERE p.owner_id = ? AND p.pairing_id = ?`, ownerID, pairingID).
		Scan(&expiresMs, &requestID, &label, &code, &approvedMs, &exchangedMs)
	if errors.Is(err, sql.ErrNoRows) {
		return Pairing{}, ErrNotFound
	}
	if err != nil {
		return Pairing{}, fmt.Errorf("read pairing: %w", err)
	}
	result := Pairing{ID: pairingID, ExpiresAt: time.UnixMilli(expiresMs).UTC()}
	if requestID.Valid {
		status := "pending"
		if approvedMs.Valid {
			status = "approved"
		}
		if exchangedMs.Valid {
			status = "exchanged"
		}
		result.Request = &Request{ID: requestID.String, DeviceLabel: label.String,
			ComparisonCode: code.String, Status: status}
	}
	return result, nil
}

func (s Store) RequestPairing(ctx context.Context, pairingID, label string, now time.Time) (Request, error) {
	if !validLabel(label) || !uuidPattern.MatchString(pairingID) {
		return Request{}, ErrDenied
	}
	requestID, err := randomID()
	if err != nil {
		return Request{}, err
	}
	claim, digest, err := randomToken("pr_")
	if err != nil {
		return Request{}, err
	}
	code, err := randomCode()
	if err != nil {
		return Request{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Request{}, fmt.Errorf("begin pairing request: %w", err)
	}
	defer tx.Rollback()
	var expiresMs int64
	var usedMs sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT expires_at_ms, used_at_ms FROM device_pairings
		WHERE pairing_id = ?`, pairingID).Scan(&expiresMs, &usedMs)
	if errors.Is(err, sql.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	if err != nil {
		return Request{}, fmt.Errorf("read pairing invitation: %w", err)
	}
	if now.UTC().UnixMilli() >= expiresMs {
		return Request{}, ErrExpired
	}
	if usedMs.Valid {
		return Request{}, ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO pairing_requests
		(request_id, pairing_id, claim_hash, comparison_code, device_label)
		VALUES (?, ?, ?, ?, ?)`, requestID, pairingID, digest[:], code, label)
	if err != nil {
		return Request{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return Request{}, fmt.Errorf("commit pairing request: %w", err)
	}
	return Request{ID: requestID, DeviceLabel: label, ComparisonCode: code,
		Status: "pending", ClaimToken: claim}, nil
}

func (s Store) ApprovePairing(ctx context.Context, ownerID, pairingID, requestID, code string, now time.Time) error {
	if !uuidPattern.MatchString(pairingID) || !uuidPattern.MatchString(requestID) || len(code) != 6 {
		return ErrDenied
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin pairing approval: %w", err)
	}
	defer tx.Rollback()
	var stored string
	var attempts int
	var expiresMs int64
	var approvedMs, usedMs sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT r.comparison_code, r.failed_attempts,
		p.expires_at_ms, r.approved_at_ms, p.used_at_ms
		FROM pairing_requests r JOIN device_pairings p ON p.pairing_id = r.pairing_id
		WHERE p.owner_id = ? AND p.pairing_id = ? AND r.request_id = ?`,
		ownerID, pairingID, requestID).Scan(&stored, &attempts, &expiresMs, &approvedMs, &usedMs)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read pairing approval: %w", err)
	}
	if now.UTC().UnixMilli() >= expiresMs {
		return ErrExpired
	}
	if approvedMs.Valid || usedMs.Valid || attempts >= 3 {
		return ErrConflict
	}
	if subtle.ConstantTimeCompare([]byte(code), []byte(stored)) != 1 {
		if _, err := tx.ExecContext(ctx, `UPDATE pairing_requests SET failed_attempts = failed_attempts + 1
			WHERE request_id = ?`, requestID); err != nil {
			return fmt.Errorf("record comparison failure: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit comparison failure: %w", err)
		}
		return ErrCode
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pairing_requests SET approved_at_ms = ?
		WHERE request_id = ?`, now.UTC().UnixMilli(), requestID); err != nil {
		return fmt.Errorf("approve pairing: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit pairing approval: %w", err)
	}
	return nil
}

func (s Store) ExchangePairing(ctx context.Context, pairingID, requestID, claim string, now time.Time) (Device, error) {
	if !uuidPattern.MatchString(pairingID) || !uuidPattern.MatchString(requestID) || !validToken(claim, "pr_") {
		return Device{}, ErrDenied
	}
	deviceID, err := randomID()
	if err != nil {
		return Device{}, err
	}
	token, credentialHash, err := randomToken("dr_")
	if err != nil {
		return Device{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Device{}, fmt.Errorf("begin pairing exchange: %w", err)
	}
	defer tx.Rollback()
	var ownerID, label string
	var storedHash []byte
	var expiresMs int64
	var approvedMs, usedMs sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT p.owner_id, r.device_label, r.claim_hash,
		p.expires_at_ms, r.approved_at_ms, p.used_at_ms
		FROM pairing_requests r JOIN device_pairings p ON p.pairing_id = r.pairing_id
		WHERE p.pairing_id = ? AND r.request_id = ?`, pairingID, requestID).
		Scan(&ownerID, &label, &storedHash, &expiresMs, &approvedMs, &usedMs)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	if err != nil {
		return Device{}, fmt.Errorf("read pairing exchange: %w", err)
	}
	if now.UTC().UnixMilli() >= expiresMs {
		return Device{}, ErrExpired
	}
	if usedMs.Valid {
		return Device{}, ErrConflict
	}
	if !approvedMs.Valid {
		return Device{}, ErrDenied
	}
	claimHash := sha256.Sum256([]byte(claim))
	if subtle.ConstantTimeCompare(claimHash[:], storedHash) != 1 {
		return Device{}, ErrDenied
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO devices
		(device_id, owner_id, device_label, credential_hash, created_at_ms)
		VALUES (?, ?, ?, ?, ?)`, deviceID, ownerID, label, credentialHash[:], now.UTC().UnixMilli()); err != nil {
		return Device{}, fmt.Errorf("create device: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pairing_requests SET exchanged_at_ms = ?
		WHERE request_id = ?`, now.UTC().UnixMilli(), requestID); err != nil {
		return Device{}, fmt.Errorf("consume pairing request: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE device_pairings SET used_at_ms = ?
		WHERE pairing_id = ?`, now.UTC().UnixMilli(), pairingID); err != nil {
		return Device{}, fmt.Errorf("consume pairing invitation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Device{}, fmt.Errorf("commit pairing exchange: %w", err)
	}
	return Device{ID: deviceID, Label: label, DeviceToken: token}, nil
}

func (s Store) ListDevices(ctx context.Context, ownerID string) ([]Device, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT device_id, device_label, revoked_at_ms
		FROM devices WHERE owner_id = ? ORDER BY created_at_ms DESC LIMIT 100`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()
	devices := make([]Device, 0)
	for rows.Next() {
		var d Device
		var revoked sql.NullInt64
		if err := rows.Scan(&d.ID, &d.Label, &revoked); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		d.Revoked = revoked.Valid
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s Store) RevokeDevice(ctx context.Context, ownerID, deviceID string, now time.Time) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE devices SET revoked_at_ms = ?
		WHERE owner_id = ? AND device_id = ? AND revoked_at_ms IS NULL`,
		now.UTC().UnixMilli(), ownerID, deviceID)
	if err != nil {
		return fmt.Errorf("revoke device: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrNotFound
	}
	return nil
}

func (s Store) CreatePing(ctx context.Context, ownerID, deviceID string, now time.Time) (Ping, error) {
	id, err := randomID()
	if err != nil {
		return Ping{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Ping{}, fmt.Errorf("begin synthetic ping: %w", err)
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM devices
		WHERE owner_id = ? AND device_id = ? AND revoked_at_ms IS NULL`, ownerID, deviceID).Scan(&active); err != nil {
		return Ping{}, fmt.Errorf("check target device: %w", err)
	}
	if active != 1 {
		return Ping{}, ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO synthetic_pings
		(ping_id, owner_id, device_id, created_at_ms) VALUES (?, ?, ?, ?)`,
		id, ownerID, deviceID, now.UTC().UnixMilli()); err != nil {
		return Ping{}, fmt.Errorf("create synthetic ping: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Ping{}, fmt.Errorf("commit synthetic ping: %w", err)
	}
	return Ping{ID: id, DeviceID: deviceID, CreatedAt: now.UTC()}, nil
}

func (s Store) GetPing(ctx context.Context, ownerID, pingID string) (Ping, error) {
	var result Ping
	var createdMs int64
	var ackMs sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT ping_id, device_id, created_at_ms, acknowledged_at_ms
		FROM synthetic_pings WHERE owner_id = ? AND ping_id = ?`, ownerID, pingID).
		Scan(&result.ID, &result.DeviceID, &createdMs, &ackMs)
	if errors.Is(err, sql.ErrNoRows) {
		return Ping{}, ErrNotFound
	}
	if err != nil {
		return Ping{}, fmt.Errorf("read synthetic ping: %w", err)
	}
	result.CreatedAt = time.UnixMilli(createdMs).UTC()
	if ackMs.Valid {
		at := time.UnixMilli(ackMs.Int64).UTC()
		result.AcknowledgedAt = &at
	}
	return result, nil
}

func (s Store) ListEvents(ctx context.Context, ownerID string) ([]Event, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT device_id, client_event_id, received_at_ms
		FROM synthetic_events WHERE owner_id = ? ORDER BY received_at_ms DESC, client_event_id DESC LIMIT 100`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list synthetic events: %w", err)
	}
	defer rows.Close()
	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		var receivedMs int64
		if err := rows.Scan(&event.DeviceID, &event.ClientEventID, &receivedMs); err != nil {
			return nil, fmt.Errorf("scan synthetic event: %w", err)
		}
		event.ReceivedAt = time.UnixMilli(receivedMs).UTC()
		events = append(events, event)
	}
	return events, rows.Err()
}

// Sync commits an optional synthetic event and ping acknowledgement before it
// returns any pending pings. A revoked credential fails inside the write tx.
func (s Store) Sync(ctx context.Context, token, eventID, ackPingID string, now time.Time) (SyncResult, error) {
	if !validToken(token, "dr_") || (eventID != "" && !uuidPattern.MatchString(eventID)) ||
		(ackPingID != "" && !uuidPattern.MatchString(ackPingID)) {
		return SyncResult{}, ErrDenied
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return SyncResult{}, fmt.Errorf("begin synthetic sync: %w", err)
	}
	defer tx.Rollback()
	digest := sha256.Sum256([]byte(token))
	var ownerID, deviceID string
	err = tx.QueryRowContext(ctx, `SELECT owner_id, device_id FROM devices
		WHERE credential_hash = ? AND revoked_at_ms IS NULL`, digest[:]).Scan(&ownerID, &deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncResult{}, ErrDenied
	}
	if err != nil {
		return SyncResult{}, fmt.Errorf("verify device credential: %w", err)
	}
	result := SyncResult{Pings: make([]Ping, 0)}
	if eventID != "" {
		inserted, err := tx.ExecContext(ctx, `INSERT INTO synthetic_events
			(owner_id, device_id, client_event_id, received_at_ms) VALUES (?, ?, ?, ?)
			ON CONFLICT(device_id, client_event_id) DO NOTHING`,
			ownerID, deviceID, eventID, now.UTC().UnixMilli())
		if err != nil {
			return SyncResult{}, fmt.Errorf("store synthetic event: %w", err)
		}
		count, err := inserted.RowsAffected()
		if err != nil {
			return SyncResult{}, err
		}
		result.EventStatus = "duplicate"
		if count == 1 {
			result.EventStatus = "accepted"
		}
	}
	if ackPingID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM synthetic_pings
			WHERE ping_id = ? AND owner_id = ? AND device_id = ?`,
			ackPingID, ownerID, deviceID).Scan(&count); err != nil {
			return SyncResult{}, fmt.Errorf("check synthetic ping: %w", err)
		}
		if count != 1 {
			return SyncResult{}, ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `UPDATE synthetic_pings SET acknowledged_at_ms = ?
			WHERE ping_id = ? AND owner_id = ? AND device_id = ? AND acknowledged_at_ms IS NULL`,
			now.UTC().UnixMilli(), ackPingID, ownerID, deviceID); err != nil {
			return SyncResult{}, fmt.Errorf("acknowledge synthetic ping: %w", err)
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT ping_id, created_at_ms FROM synthetic_pings
		WHERE owner_id = ? AND device_id = ? AND acknowledged_at_ms IS NULL
		ORDER BY created_at_ms, ping_id LIMIT 10`, ownerID, deviceID)
	if err != nil {
		return SyncResult{}, fmt.Errorf("read pending synthetic pings: %w", err)
	}
	for rows.Next() {
		var ping Ping
		var createdMs int64
		if err := rows.Scan(&ping.ID, &createdMs); err != nil {
			rows.Close()
			return SyncResult{}, fmt.Errorf("scan pending synthetic ping: %w", err)
		}
		ping.DeviceID = deviceID
		ping.CreatedAt = time.UnixMilli(createdMs).UTC()
		result.Pings = append(result.Pings, ping)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return SyncResult{}, fmt.Errorf("iterate pending synthetic pings: %w", rowsErr)
	}
	if err := tx.Commit(); err != nil {
		return SyncResult{}, fmt.Errorf("commit synthetic sync: %w", err)
	}
	return result, nil
}

func validLabel(value string) bool {
	if len(value) < 1 || len(value) > 80 || !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func randomID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("random identifier unavailable")
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:], nil
}

func randomToken(prefix string) (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, errors.New("random credential unavailable")
	}
	token := prefix + base64.RawURLEncoding.EncodeToString(raw[:])
	return token, sha256.Sum256([]byte(token)), nil
}

func validToken(token, prefix string) bool {
	if !strings.HasPrefix(token, prefix) || len(token) != len(prefix)+43 {
		return false
	}
	encoded := strings.TrimPrefix(token, prefix)
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == encoded
}

func randomCode() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("comparison code unavailable")
	}
	value := uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])
	return fmt.Sprintf("%06d", value%1000000), nil
}
