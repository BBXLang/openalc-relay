package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

const sessionLifetime = 12 * time.Hour

// CreateSession returns an opaque bearer token. The database stores only its
// digest; callers must put the token in a Secure, HttpOnly browser cookie.
func CreateSession(ctx context.Context, db *sql.DB, ownerID string, now time.Time) (string, time.Time, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", time.Time{}, errors.New("session randomness unavailable")
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	digest := sha256.Sum256([]byte(token))
	expires := now.UTC().Add(sessionLifetime)
	_, err := db.ExecContext(ctx, `INSERT INTO web_sessions
		(token_hash, owner_id, created_at_ms, expires_at_ms)
		VALUES (?, ?, ?, ?)`, digest[:], ownerID, now.UTC().UnixMilli(), expires.UnixMilli())
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create web session: %w", err)
	}
	return token, expires, nil
}

// LookupSession checks revocation and expiry on every request.
func LookupSession(ctx context.Context, db *sql.DB, token string, now time.Time) (string, time.Time, bool, error) {
	if !validSessionToken(token) {
		return "", time.Time{}, false, nil
	}
	digest := sha256.Sum256([]byte(token))
	var ownerID string
	var expiresMs int64
	err := db.QueryRowContext(ctx, `SELECT owner_id, expires_at_ms FROM web_sessions
		WHERE token_hash = ? AND revoked_at_ms IS NULL AND expires_at_ms > ?`,
		digest[:], now.UTC().UnixMilli()).Scan(&ownerID, &expiresMs)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("lookup web session: %w", err)
	}
	return ownerID, time.UnixMilli(expiresMs).UTC(), true, nil
}

func RevokeSession(ctx context.Context, db *sql.DB, token string, now time.Time) error {
	if !validSessionToken(token) {
		return nil
	}
	digest := sha256.Sum256([]byte(token))
	_, err := db.ExecContext(ctx, `UPDATE web_sessions SET revoked_at_ms = ?
		WHERE token_hash = ? AND revoked_at_ms IS NULL`, now.UTC().UnixMilli(), digest[:])
	if err != nil {
		return fmt.Errorf("revoke web session: %w", err)
	}
	return nil
}

func validSessionToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == token
}
