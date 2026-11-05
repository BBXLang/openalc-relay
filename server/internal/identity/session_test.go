package identity

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

func TestSessionRevocationAndExpiry(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Bootstrap(ctx, db, []byte("local owner passphrase")); err != nil {
		t.Fatal(err)
	}
	ownerID, ok, err := VerifyPassword(ctx, db, []byte("local owner passphrase"))
	if err != nil || !ok {
		t.Fatalf("owner login failed: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	token, expires, err := CreateSession(ctx, db, ownerID, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := LookupSession(ctx, db, "forged", now); err != nil || ok {
		t.Fatalf("forged token accepted: %v", err)
	}
	gotID, gotExpiry, ok, err := LookupSession(ctx, db, token, now)
	if err != nil || !ok || gotID != ownerID || !gotExpiry.Equal(expires) {
		t.Fatalf("valid session rejected: %v", err)
	}
	if _, _, ok, err := LookupSession(ctx, db, token, expires); err != nil || ok {
		t.Fatalf("expired session accepted: %v", err)
	}
	if err := RevokeSession(ctx, db, token, now); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := LookupSession(ctx, db, token, now); err != nil || ok {
		t.Fatalf("revoked session accepted: %v", err)
	}
}
