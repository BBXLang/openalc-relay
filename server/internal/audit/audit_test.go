package audit

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

func testStore(t *testing.T) (Store, string) {
	t.Helper()
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := identity.Bootstrap(context.Background(), db, []byte("synthetic owner password")); err != nil {
		t.Fatal(err)
	}
	ownerID, ok, err := identity.VerifyPassword(context.Background(), db, []byte("synthetic owner password"))
	if err != nil || !ok {
		t.Fatalf("read owner: %v", err)
	}
	return Store{DB: db}, ownerID
}

func TestAppendAndList(t *testing.T) {
	store, ownerID := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	entry := Entry{
		OwnerID: ownerID, ActorKind: ActorOwnerSession, ActorID: ownerID,
		Action: "mail.account.create", SubjectKind: "mail_account", SubjectID: "account-1",
		Detail: map[string]string{"connection_state": "untested"},
	}
	if err := store.Append(ctx, entry, now); err != nil {
		t.Fatal(err)
	}
	entries, err := store.List(ctx, ownerID, 50)
	if err != nil || len(entries) != 1 {
		t.Fatalf("list audit entries: %d, %v", len(entries), err)
	}
	stored := entries[0]
	if stored.ID == "" || stored.Action != "mail.account.create" || stored.CreatedAt.IsZero() {
		t.Fatalf("entry fields missing: %+v", stored)
	}
	if stored.Detail["connection_state"] != "untested" {
		t.Fatalf("detail metadata lost: %+v", stored.Detail)
	}
	if _, ok := stored.Detail["password"]; ok {
		t.Fatal("audit detail must stay metadata only")
	}
	// Cross-owner data is invisible.
	other, err := store.List(ctx, "other-owner", 50)
	if err != nil || len(other) != 0 {
		t.Fatalf("cross-owner audit visible: %d, %v", len(other), err)
	}
}

func TestAppendValidationFailsClosed(t *testing.T) {
	store, ownerID := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	valid := Entry{
		OwnerID: ownerID, ActorKind: ActorDevice, ActorID: "device-1",
		Action: "sms.command.claim", SubjectKind: "sms_command", SubjectID: "command-1",
	}
	cases := map[string]func(*Entry){
		"unknown_actor_kind": func(e *Entry) { e.ActorKind = "anonymous" },
		"missing_owner":      func(e *Entry) { e.OwnerID = "" },
		"oversized_detail": func(e *Entry) {
			e.Detail = map[string]string{"blob": strings.Repeat("a", MaxDetailBytes)}
		},
		"nul_in_detail": func(e *Entry) { e.Detail = map[string]string{"k": "a\x00b"} },
		"empty_action":  func(e *Entry) { e.Action = "" },
	}
	for name, mutate := range cases {
		entry := valid
		mutate(&entry)
		if err := store.Append(ctx, entry, now); err == nil {
			t.Fatalf("invalid entry accepted: %s", name)
		}
	}
	entries, err := store.List(ctx, ownerID, 50)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected entries must not persist: %d, %v", len(entries), err)
	}
}
