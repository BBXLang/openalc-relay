package mailstore

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

type fixture struct {
	store    Store
	ownerID  string
	keyPath  string
	dbPath   string
	baseTime time.Time
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	keyPath := filepath.Join(t.TempDir(), "keys.json")
	if err := secrets.CreateFile(keyPath); err != nil {
		t.Fatal(err)
	}
	keys, err := secrets.LoadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "relay.db")
	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := identity.Bootstrap(ctx, db, []byte("synthetic owner password")); err != nil {
		t.Fatal(err)
	}
	ownerID, ok, err := identity.VerifyPassword(ctx, db, []byte("synthetic owner password"))
	if err != nil || !ok {
		t.Fatalf("read owner: %v", err)
	}
	return fixture{
		store:    Store{DB: db, Keys: keys},
		ownerID:  ownerID,
		keyPath:  keyPath,
		dbPath:   dbPath,
		baseTime: time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
	}
}

func (f fixture) account(t *testing.T) Account {
	t.Helper()
	account, err := f.store.CreateAccount(context.Background(), f.ownerID, CreateAccountInput{
		Address: "user@example.test", IMAPHost: "imap.example.test", IMAPPort: 993,
		SMTPHost: "smtp.example.test", SMTPPort: 465, TLSMode: TLSImplicitTLS,
		Username: "user@example.test", Password: "synthetic-app-password",
	}, f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func (f fixture) input(accountID, folder, remoteID, subject, body string, receivedAt time.Time) MessageInput {
	return MessageInput{
		AccountID: accountID, Folder: folder, RemoteMessageID: remoteID,
		ReceivedAt: receivedAt, SizeBytes: int64(len(body)), Unread: true,
		HasAttachments: false, BodyFormat: BodyTextPlain, Subject: subject,
		Headers: MessageHeaders{From: "sender@example.test", To: []string{"user@example.test"}},
		Body:    body,
	}
}

func countRows(t *testing.T, store Store, table string) int {
	t.Helper()
	if !map[string]bool{"mail_messages": true, "mail_attachments": true, "audit_entries": true}[table] {
		t.Fatalf("unexpected table %s", table)
	}
	var count int
	if err := store.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestAccountLifecycleAndScoping(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	if account.ConnectionState != ConnectionUntested || account.Revoked || account.LastSyncAt != nil {
		t.Fatalf("unexpected initial account state: %+v", account)
	}
	encoded, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(encoded)), "password") ||
		strings.Contains(strings.ToLower(string(encoded)), "credential") {
		t.Fatalf("account representation leaks credential fields: %s", encoded)
	}
	if _, err := f.store.GetAccount(ctx, "other-owner", account.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner account read: %v", err)
	}
	if err := f.store.SetConnectionResult(ctx, f.ownerID, account.ID, true, f.baseTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	updated, err := f.store.GetAccount(ctx, f.ownerID, account.ID)
	if err != nil || updated.ConnectionState != ConnectionOK || updated.LastSyncAt == nil {
		t.Fatalf("connection result not recorded: %+v, %v", updated, err)
	}
	accounts, err := f.store.ListAccounts(ctx, f.ownerID)
	if err != nil || len(accounts) != 1 || accounts[0].ID != account.ID || accounts[0].Address != "user@example.test" {
		t.Fatalf("list accounts: %+v, %v", accounts, err)
	}
	if foreign, err := f.store.ListAccounts(ctx, "other-owner"); err != nil || len(foreign) != 0 {
		t.Fatalf("cross-owner account list: %+v, %v", foreign, err)
	}
	if _, err := f.store.InsertMessage(ctx, f.ownerID, f.input(account.ID, "INBOX", "<r1@example.test>",
		"subject", "body", f.baseTime), f.baseTime); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RevokeAccount(ctx, f.ownerID, account.ID, f.baseTime.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RevokeAccount(ctx, f.ownerID, account.ID, f.baseTime); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double revoke: %v", err)
	}
	if _, err := f.store.InsertMessage(ctx, f.ownerID, f.input(account.ID, "INBOX", "<r2@example.test>",
		"subject", "body", f.baseTime), f.baseTime); !errors.Is(err, ErrDenied) {
		t.Fatalf("revoked account still accepts imports: %v", err)
	}
	if err := f.store.DeleteAccount(ctx, f.ownerID, account.ID, f.baseTime); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.GetAccount(ctx, f.ownerID, account.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted account readable: %v", err)
	}
	if count := countRows(t, f.store, "mail_messages"); count != 0 {
		t.Fatalf("messages survived account deletion: %d", count)
	}
	if _, err := f.store.CreateAccount(ctx, f.ownerID, CreateAccountInput{
		Address: "bad", IMAPHost: "imap.example.test", IMAPPort: 993,
		SMTPHost: "smtp.example.test", SMTPPort: 465, TLSMode: "plain",
		Username: "u", Password: "p",
	}, f.baseTime); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid account accepted: %v", err)
	}
}

func TestMailImportDedupAndConflict(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	input := f.input(account.ID, "INBOX", "<dup@example.test>", "weekly report", "synthetic body", f.baseTime)
	if status, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil || status != StatusAccepted {
		t.Fatalf("first import: %s, %v", status, err)
	}
	if status, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime.Add(time.Minute)); err != nil || status != StatusDuplicate {
		t.Fatalf("repeated import: %s, %v", status, err)
	}
	if count := countRows(t, f.store, "mail_messages"); count != 1 {
		t.Fatalf("duplicate imported twice: %d", count)
	}
	// Same key, different content: explicit conflict, stored record untouched.
	clash := input
	clash.Body = "tampered synthetic body"
	if _, err := f.store.InsertMessage(ctx, f.ownerID, clash, f.baseTime); !errors.Is(err, ErrConflict) {
		t.Fatalf("same key different content: %v", err)
	}
	stored, err := f.store.GetMessage(ctx, f.ownerID, f.onlyMessageID(t))
	if err != nil || stored.Body != "synthetic body" {
		t.Fatalf("stored record changed: %+v, %v", stored, err)
	}
	// The same content under a different key is a new message.
	other := input
	other.RemoteMessageID = "<other@example.test>"
	if status, err := f.store.InsertMessage(ctx, f.ownerID, other, f.baseTime); err != nil || status != StatusAccepted {
		t.Fatalf("different key import: %s, %v", status, err)
	}
	if count := countRows(t, f.store, "mail_messages"); count != 2 {
		t.Fatalf("message count: %d", count)
	}
	// A nonexistent or foreign account is never writable.
	foreign := input
	foreign.AccountID = "missing"
	if _, err := f.store.InsertMessage(ctx, "other-owner", foreign, f.baseTime); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner import: %v", err)
	}
}

func (f fixture) onlyMessageID(t *testing.T) string {
	t.Helper()
	var id string
	if err := f.store.DB.QueryRow("SELECT message_id FROM mail_messages").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTamperedCiphertextFailsClosed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	input := f.input(account.ID, "INBOX", "<t1@example.test>", "subject", "synthetic body", f.baseTime)
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil {
		t.Fatal(err)
	}
	messageID := f.onlyMessageID(t)
	for _, field := range []string{"subject_ciphertext", "headers_ciphertext", "body_ciphertext"} {
		var stored []byte
		if err := f.store.DB.QueryRow("SELECT "+field+" FROM mail_messages WHERE message_id = ?",
			messageID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		stored[len(stored)-1] ^= 1
		if _, err := f.store.DB.Exec("UPDATE mail_messages SET "+field+" = ? WHERE message_id = ?",
			stored, messageID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.GetMessage(ctx, f.ownerID, messageID); !errors.Is(err, ErrCiphertext) {
			t.Fatalf("tampered %s opened: %v", field, err)
		}
		if _, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{}); !errors.Is(err, ErrCiphertext) {
			t.Fatalf("tampered %s listed: %v", field, err)
		}
		if _, err := f.store.SearchMessages(ctx, f.ownerID, SearchOptions{Query: "synthetic"}); !errors.Is(err, ErrCiphertext) {
			t.Fatalf("tampered %s searched: %v", field, err)
		}
		// Restore the field for the next iteration.
		stored[len(stored)-1] ^= 1
		if _, err := f.store.DB.Exec("UPDATE mail_messages SET "+field+" = ? WHERE message_id = ?",
			stored, messageID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.GetMessage(ctx, f.ownerID, messageID); err != nil {
		t.Fatalf("restored record unreadable: %v", err)
	}
}

func TestWrongKeyringFailsClosed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	input := f.input(account.ID, "INBOX", "<k1@example.test>", "subject", "synthetic body", f.baseTime)
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil {
		t.Fatal(err)
	}
	otherPath := filepath.Join(t.TempDir(), "other-keys.json")
	if err := secrets.CreateFile(otherPath); err != nil {
		t.Fatal(err)
	}
	wrongKeys, err := secrets.LoadFile(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	locked := Store{DB: f.store.DB, Keys: wrongKeys}
	if _, err := locked.GetMessage(ctx, f.ownerID, f.onlyMessageID(t)); !errors.Is(err, ErrCiphertext) {
		t.Fatalf("wrong keyring decrypted message: %v", err)
	}
	// The deduplication path must also fail closed instead of reporting duplicate.
	retry := f.input(account.ID, "INBOX", "<k1@example.test>", "subject", "synthetic body", f.baseTime)
	if _, err := locked.InsertMessage(ctx, f.ownerID, retry, f.baseTime); !errors.Is(err, ErrCiphertext) {
		t.Fatalf("wrong keyring dedup check: %v", err)
	}
	if _, err := locked.ListMessages(ctx, f.ownerID, ListOptions{}); !errors.Is(err, ErrCiphertext) {
		t.Fatalf("wrong keyring listed messages: %v", err)
	}
}

func TestCursorPaginationStability(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	ids := make([]string, 0, 5)
	for i := range 5 {
		receivedAt := f.baseTime.Add(time.Duration(i) * time.Minute)
		input := f.input(account.ID, "INBOX", "<p"+string(rune('a'+i))+"@example.test>",
			"msg "+string(rune('a'+i)), "synthetic body "+string(rune('a'+i)), receivedAt)
		if _, err := f.store.InsertMessage(ctx, f.ownerID, input, receivedAt); err != nil {
			t.Fatal(err)
		}
		var id string
		if err := f.store.DB.QueryRow(`SELECT message_id FROM mail_messages
			WHERE remote_message_id = ?`, input.RemoteMessageID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	first, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{Limit: 2})
	if err != nil || len(first.Messages) != 2 || first.NextCursor == "" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	if first.Messages[0].ID != ids[4] || first.Messages[1].ID != ids[3] {
		t.Fatalf("first page order: %+v", first.Messages)
	}
	// New records arrive after the cursor was issued: the old window must not move.
	later := f.input(account.ID, "INBOX", "<new@example.test>", "later", "new body", f.baseTime.Add(10*time.Minute))
	if _, err := f.store.InsertMessage(ctx, f.ownerID, later, f.baseTime.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	second, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{Limit: 2, Cursor: first.NextCursor})
	if err != nil || len(second.Messages) != 2 {
		t.Fatalf("second page: %+v, %v", second, err)
	}
	if second.Messages[0].ID != ids[2] || second.Messages[1].ID != ids[1] {
		t.Fatalf("second page shifted by newer record: %+v", second.Messages)
	}
	third, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{Limit: 2, Cursor: second.NextCursor})
	if err != nil || len(third.Messages) != 1 || third.Messages[0].ID != ids[0] || third.NextCursor != "" {
		t.Fatalf("third page: %+v, %v", third, err)
	}
	filtered, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{Folder: "Archive"})
	if err != nil || len(filtered.Messages) != 0 {
		t.Fatalf("folder filter: %+v, %v", filtered, err)
	}
	byAccount, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{AccountID: "missing"})
	if err != nil || len(byAccount.Messages) != 0 {
		t.Fatalf("account filter: %+v, %v", byAccount, err)
	}
	if _, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{Cursor: "broken"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid cursor accepted: %v", err)
	}
}

func TestRestartSurvival(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	input := f.input(account.ID, "INBOX", "<r1@example.test>", "subject", "synthetic body", f.baseTime)
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil {
		t.Fatal(err)
	}
	attachment, err := f.store.AddAttachment(ctx, f.ownerID, f.onlyMessageID(t), AttachmentInput{
		Filename: "notes.txt", MimeType: "text/plain", SizeBytes: 11,
	}, f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.StoreAttachmentContent(ctx, f.ownerID, attachment.ID, []byte("synthetic data"), f.baseTime); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RevokeAccount(ctx, "other-owner", account.ID, f.baseTime); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner revoke: %v", err)
	}
	messageID := f.onlyMessageID(t)
	if err := f.store.SetConnectionResult(ctx, f.ownerID, account.ID, true, f.baseTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := f.store.DB.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopening reruns the migrations: this also covers repeated startup.
	keys, err := secrets.LoadFile(f.keyPath)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(ctx, f.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	revived := Store{DB: reopened, Keys: keys}
	message, err := revived.GetMessage(ctx, f.ownerID, messageID)
	if err != nil || message.Subject != "subject" || message.Body != "synthetic body" {
		t.Fatalf("message lost after restart: %+v, %v", message, err)
	}
	content, err := revived.GetAttachmentContent(ctx, f.ownerID, attachment.ID)
	if err != nil || string(content) != "synthetic data" {
		t.Fatalf("attachment lost after restart: %v", err)
	}
	state, err := revived.GetAccount(ctx, f.ownerID, account.ID)
	if err != nil || state.ConnectionState != ConnectionOK {
		t.Fatalf("account state lost after restart: %+v, %v", state, err)
	}
}

func TestImportRollbackOnAuditFailure(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	input := f.input(account.ID, "INBOX", "<rb1@example.test>", "subject", "synthetic body", f.baseTime)
	// Inject a deterministic failure after the message insert: the audit write
	// inside the same transaction aborts, so no partial state may survive.
	if _, err := f.store.DB.Exec(`CREATE TRIGGER fail_mail_import BEFORE INSERT ON audit_entries
		WHEN NEW.action = 'mail.message.import' BEGIN SELECT RAISE(ABORT, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err == nil {
		t.Fatal("import succeeded despite injected audit failure")
	}
	if count := countRows(t, f.store, "mail_messages"); count != 0 {
		t.Fatalf("failed import left partial writes: %d", count)
	}
	// A duplicate import whose post-comparison audit fails must also roll back.
	if _, err := f.store.DB.Exec("DROP TRIGGER fail_mail_import"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB.Exec(`CREATE TRIGGER fail_mail_import BEFORE INSERT ON audit_entries
		WHEN NEW.action = 'mail.message.import' AND NEW.subject_id = (SELECT subject_id FROM audit_entries
		WHERE action = 'mail.message.import' LIMIT 1) BEGIN SELECT RAISE(ABORT, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err == nil {
		t.Fatal("duplicate import succeeded despite injected audit failure")
	}
	if count := countRows(t, f.store, "mail_messages"); count != 1 {
		t.Fatalf("message count after failed duplicate: %d", count)
	}
}

func TestMailSearchScoping(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	other, err := f.store.CreateAccount(ctx, f.ownerID, CreateAccountInput{
		Address: "other@example.test", IMAPHost: "imap.example.test", IMAPPort: 993,
		SMTPHost: "smtp.example.test", SMTPPort: 465, TLSMode: TLSStartTLS,
		Username: "other", Password: "synthetic-app-password",
	}, f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	target := f.input(account.ID, "INBOX", "<s1@example.test>", "quarterly report",
		"the deployment window opens next week", f.baseTime)
	if _, err := f.store.InsertMessage(ctx, f.ownerID, target, f.baseTime); err != nil {
		t.Fatal(err)
	}
	unrelated := f.input(other.ID, "INBOX", "<s2@example.test>", "lunch menu", "soup today", f.baseTime)
	if _, err := f.store.InsertMessage(ctx, f.ownerID, unrelated, f.baseTime); err != nil {
		t.Fatal(err)
	}
	hits, err := f.store.SearchMessages(ctx, f.ownerID, SearchOptions{Query: "deployment window"})
	if err != nil || len(hits) != 1 || hits[0].Subject != "quarterly report" {
		t.Fatalf("body hit: %+v, %v", hits, err)
	}
	hits, err = f.store.SearchMessages(ctx, f.ownerID, SearchOptions{Query: "QUARTERLY"})
	if err != nil || len(hits) != 1 {
		t.Fatalf("case-insensitive subject hit: %+v, %v", hits, err)
	}
	if misses, err := f.store.SearchMessages(ctx, f.ownerID, SearchOptions{Query: "soup"}); err != nil || len(misses) != 1 {
		t.Fatalf("other account content searchable: %+v, %v", misses, err)
	}
	scoped, err := f.store.SearchMessages(ctx, f.ownerID, SearchOptions{AccountID: other.ID, Query: "deployment"})
	if err != nil || len(scoped) != 0 {
		t.Fatalf("account scope leaked into search: %+v, %v", scoped, err)
	}
	if leaked, err := f.store.SearchMessages(ctx, "other-owner", SearchOptions{Query: "deployment"}); err != nil || len(leaked) != 0 {
		t.Fatalf("owner scope leaked into search: %+v, %v", leaked, err)
	}
	if _, err := f.store.SearchMessages(ctx, f.ownerID, SearchOptions{Query: "   "}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty query accepted: %v", err)
	}
}

func TestAttachmentLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.account(t)
	input := f.input(account.ID, "INBOX", "<a1@example.test>", "report", "synthetic body", f.baseTime)
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil {
		t.Fatal(err)
	}
	messageID := f.onlyMessageID(t)
	attachment, err := f.store.AddAttachment(ctx, f.ownerID, messageID, AttachmentInput{
		Filename: "data.csv", MimeType: "text/csv", SizeBytes: 4,
	}, f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if attachment.DownloadState != DownloadRemoteOnly || attachment.AccountID != account.ID {
		t.Fatalf("unexpected attachment state: %+v", attachment)
	}
	if _, err := f.store.GetAttachmentContent(ctx, f.ownerID, attachment.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("undownloaded content available: %v", err)
	}
	if err := f.store.StoreAttachmentContent(ctx, "other-owner", attachment.ID, []byte("x"), f.baseTime); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner content write: %v", err)
	}
	if err := f.store.StoreAttachmentContent(ctx, f.ownerID, attachment.ID, nil, f.baseTime); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty content accepted: %v", err)
	}
	if err := f.store.StoreAttachmentContent(ctx, f.ownerID, attachment.ID, []byte("data"), f.baseTime); err != nil {
		t.Fatal(err)
	}
	content, err := f.store.GetAttachmentContent(ctx, f.ownerID, attachment.ID)
	if err != nil || string(content) != "data" {
		t.Fatalf("content round trip: %v", err)
	}
	listed, err := f.store.ListAttachments(ctx, f.ownerID, messageID)
	if err != nil || len(listed) != 1 || listed[0].DownloadState != DownloadStored {
		t.Fatalf("attachment list: %+v, %v", listed, err)
	}
	if _, err := f.store.GetAttachment(ctx, "other-owner", attachment.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner attachment read: %v", err)
	}
	var stored []byte
	if err := f.store.DB.QueryRow(`SELECT content_ciphertext FROM mail_attachments
		WHERE attachment_id = ?`, attachment.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	stored[len(stored)-1] ^= 1
	if _, err := f.store.DB.Exec(`UPDATE mail_attachments SET content_ciphertext = ?
		WHERE attachment_id = ?`, stored, attachment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.GetAttachmentContent(ctx, f.ownerID, attachment.ID); !errors.Is(err, ErrCiphertext) {
		t.Fatalf("tampered attachment content opened: %v", err)
	}
	if _, err := f.store.AddAttachment(ctx, f.ownerID, "missing", AttachmentInput{
		Filename: "x.txt", MimeType: "text/plain", SizeBytes: 1,
	}, f.baseTime); !errors.Is(err, ErrNotFound) {
		t.Fatalf("attachment for foreign message: %v", err)
	}
}
