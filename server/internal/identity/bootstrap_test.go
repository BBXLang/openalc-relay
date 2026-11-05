package identity

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

func TestLocalBootstrapAndPasswordVerification(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "relay.db")
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if initialized, err := IsInitialized(ctx, db); err != nil || initialized {
		t.Fatalf("new database unexpectedly initialized: %v", err)
	}
	if err := Bootstrap(ctx, db, []byte("short")); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("short password accepted: %v", err)
	}
	password := []byte("a long local test passphrase")
	if err := Bootstrap(ctx, db, password); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(ctx, db, password); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second owner created: %v", err)
	}
	ownerID, ok, err := VerifyPassword(ctx, db, password)
	if err != nil || !ok || ownerID == "" {
		t.Fatalf("correct password rejected: %v", err)
	}
	if _, ok, err := VerifyPassword(ctx, db, []byte("a wrong local passphrase")); err != nil || ok {
		t.Fatalf("wrong password accepted: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reopenedID, ok, err := VerifyPassword(ctx, db, password)
	if err != nil || !ok || reopenedID != ownerID {
		t.Fatalf("password verifier did not survive restart: %v", err)
	}
}
