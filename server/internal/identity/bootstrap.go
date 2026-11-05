// Package identity implements the local owner bootstrap and password verifier.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime      = 3
	argonMemoryKiB = 64 * 1024
	argonThreads   = 2
	maxPasswordLen = 1024
)

var (
	ErrAlreadyInitialized = errors.New("owner already initialized")
	ErrInvalidPassword    = errors.New("password must contain 12 to 1024 bytes")
)

func validPassword(password []byte) bool {
	return len(password) >= 12 && len(password) <= maxPasswordLen
}

// Bootstrap creates the only owner through a local administrative command.
// No HTTP endpoint can initialize an owner.
func Bootstrap(ctx context.Context, db *sql.DB, password []byte) error {
	if !validPassword(password) {
		return ErrInvalidPassword
	}
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return errors.New("password salt unavailable")
	}
	hash := argon2.IDKey(password, salt[:], argonTime, argonMemoryKiB, argonThreads, 32)
	ownerID, err := randomUUID()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin owner bootstrap: %w", err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM owners").Scan(&count); err != nil {
		return fmt.Errorf("check owner bootstrap: %w", err)
	}
	if count != 0 {
		return ErrAlreadyInitialized
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO owners
		(owner_id, singleton, password_salt, password_hash, argon_time, argon_memory_kib, argon_threads, created_at_ms)
		VALUES (?, 1, ?, ?, ?, ?, ?, ?)`, ownerID, salt[:], hash,
		argonTime, argonMemoryKiB, argonThreads, time.Now().UTC().UnixMilli())
	if err != nil {
		return fmt.Errorf("insert owner: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit owner bootstrap: %w", err)
	}
	return nil
}

// VerifyPassword returns only the owner ID on a successful check. The caller
// must never log the supplied password or the stored verifier.
func VerifyPassword(ctx context.Context, db *sql.DB, password []byte) (string, bool, error) {
	if len(password) > maxPasswordLen {
		return "", false, nil
	}
	var ownerID string
	var salt, expected []byte
	var timeCost, memoryKiB, threads uint32
	err := db.QueryRowContext(ctx, `SELECT owner_id, password_salt, password_hash,
		argon_time, argon_memory_kib, argon_threads FROM owners WHERE singleton = 1`).
		Scan(&ownerID, &salt, &expected, &timeCost, &memoryKiB, &threads)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read owner verifier: %w", err)
	}
	if len(salt) != 16 || len(expected) != 32 || timeCost < 1 || timeCost > 10 ||
		memoryKiB < 8192 || memoryKiB > 262144 || threads < 1 || threads > 8 {
		return "", false, errors.New("owner verifier configuration invalid")
	}
	actual := argon2.IDKey(password, salt, timeCost, memoryKiB, uint8(threads), 32)
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return "", false, nil
	}
	return ownerID, true, nil
}

func IsInitialized(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM owners").Scan(&count); err != nil {
		return false, fmt.Errorf("check owner initialization: %w", err)
	}
	return count == 1, nil
}

func randomUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("owner identifier unavailable")
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:], nil
}
