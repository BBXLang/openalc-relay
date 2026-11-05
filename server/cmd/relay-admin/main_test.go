package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

func TestLocalAdminInitialization(t *testing.T) {
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "keys.json")
	passwordPath := filepath.Join(directory, "password.txt")
	databasePath := filepath.Join(directory, "relay.db")
	if err := os.WriteFile(passwordPath, []byte("a local bootstrap passphrase\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"keygen", "-out", keyPath}); err != nil {
		t.Fatal(err)
	}
	args := []string{"bootstrap", "-database", databasePath, "-key-file", keyPath, "-password-file", passwordPath}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	if err := run(args); !errors.Is(err, identity.ErrAlreadyInitialized) {
		t.Fatalf("repeated bootstrap accepted: %v", err)
	}
	db, err := sqlite.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, ok, err := identity.VerifyPassword(context.Background(), db, []byte("a local bootstrap passphrase")); err != nil || !ok {
		t.Fatalf("bootstrap password unavailable: %v", err)
	}
}
