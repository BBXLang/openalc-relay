package smsstore

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
	"github.com/OpenALC/relay/server/internal/synthetic"
)

type fixture struct {
	store     Store
	synthetic synthetic.Store
	ownerID   string
	keyPath   string
	dbPath    string
	baseTime  time.Time
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
		store:     Store{DB: db, Keys: keys},
		synthetic: synthetic.Store{DB: db},
		ownerID:   ownerID,
		keyPath:   keyPath,
		dbPath:    dbPath,
		baseTime:  time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
	}
}

// device pairs a synthetic device, mirroring the real owner approved flow.
func (f fixture) device(t *testing.T, label string) synthetic.Device {
	t.Helper()
	ctx := context.Background()
	now := f.baseTime
	pair, err := f.synthetic.CreatePairing(ctx, f.ownerID, now)
	if err != nil {
		t.Fatal(err)
	}
	request, err := f.synthetic.RequestPairing(ctx, pair.ID, label, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.synthetic.ApprovePairing(ctx, f.ownerID, pair.ID, request.ID, request.ComparisonCode, now); err != nil {
		t.Fatal(err)
	}
	device, err := f.synthetic.ExchangePairing(ctx, pair.ID, request.ID, request.ClaimToken, now)
	if err != nil {
		t.Fatal(err)
	}
	return device
}

func (f fixture) messageInput(deviceID, remoteID, body string) MessageInput {
	return MessageInput{
		DeviceID: deviceID, Direction: DirectionInbound, RemoteEventID: remoteID,
		Sender: "100086", Recipient: "1069000000000", Body: body,
		DeviceAt: f.baseTime,
	}
}

func (f fixture) commandInput(deviceID, key, body string) CommandInput {
	return CommandInput{
		DeviceID: deviceID, IdempotencyKey: key,
		Recipient: "100087", Body: body,
	}
}

func (f fixture) smsCount(t *testing.T) int {
	t.Helper()
	var count int
	if err := f.store.DB.QueryRow("SELECT count(*) FROM sms_messages").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func (f fixture) commandCountForKey(t *testing.T, key string) int {
	t.Helper()
	var count int
	if err := f.store.DB.QueryRow(`SELECT count(*) FROM sms_send_commands
		WHERE idempotency_key = ?`, key).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestSMSImportDedupAndConflict(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	device := f.device(t, "Xiaomi Agent")
	input := f.messageInput(device.ID, "evt-1", "synthetic inbound body")
	if status, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil || status != StatusAccepted {
		t.Fatalf("first import: %s, %v", status, err)
	}
	if status, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime.Add(time.Minute)); err != nil || status != StatusDuplicate {
		t.Fatalf("repeated import: %s, %v", status, err)
	}
	if count := f.smsCount(t); count != 1 {
		t.Fatalf("duplicate stored twice: %d", count)
	}
	clash := input
	clash.Body = "different synthetic body"
	if _, err := f.store.InsertMessage(ctx, f.ownerID, clash, f.baseTime); !errors.Is(err, ErrConflict) {
		t.Fatalf("same key different content: %v", err)
	}
	stored, err := f.store.GetMessage(ctx, f.ownerID, f.onlySMSID(t))
	if err != nil || stored.Body != "synthetic inbound body" {
		t.Fatalf("stored record changed: %+v, %v", stored, err)
	}
	if _, err := f.store.GetMessage(ctx, "other-owner", stored.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner message read: %v", err)
	}
	if _, err := f.store.InsertMessage(ctx, f.ownerID, f.messageInput("missing", "evt-2", "x"), f.baseTime); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign device accepted: %v", err)
	}
	bad := input
	bad.Direction = "sideways"
	if _, err := f.store.InsertMessage(ctx, f.ownerID, bad, f.baseTime); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid direction accepted: %v", err)
	}
	if err := f.synthetic.RevokeDevice(ctx, f.ownerID, device.ID, f.baseTime); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.InsertMessage(ctx, f.ownerID, f.messageInput(device.ID, "evt-3", "x"), f.baseTime); !errors.Is(err, ErrDenied) {
		t.Fatalf("revoked device accepted: %v", err)
	}
}

func (f fixture) onlySMSID(t *testing.T) string {
	t.Helper()
	var id string
	if err := f.store.DB.QueryRow("SELECT sms_id FROM sms_messages").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSMSTamperAndWrongKeyFailsClosed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	device := f.device(t, "Xiaomi Agent")
	input := f.messageInput(device.ID, "evt-1", "synthetic inbound body")
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil {
		t.Fatal(err)
	}
	smsID := f.onlySMSID(t)
	for _, field := range []string{"sender_ciphertext", "recipient_ciphertext", "body_ciphertext"} {
		var stored []byte
		if err := f.store.DB.QueryRow("SELECT "+field+" FROM sms_messages WHERE sms_id = ?", smsID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		stored[len(stored)-1] ^= 1
		if _, err := f.store.DB.Exec("UPDATE sms_messages SET "+field+" = ? WHERE sms_id = ?", stored, smsID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.GetMessage(ctx, f.ownerID, smsID); !errors.Is(err, ErrCiphertext) {
			t.Fatalf("tampered %s opened: %v", field, err)
		}
		if _, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{}); !errors.Is(err, ErrCiphertext) {
			t.Fatalf("tampered %s listed: %v", field, err)
		}
		if _, err := f.store.Search(ctx, f.ownerID, SearchOptions{Query: "synthetic"}); !errors.Is(err, ErrCiphertext) {
			t.Fatalf("tampered %s searched: %v", field, err)
		}
		stored[len(stored)-1] ^= 1
		if _, err := f.store.DB.Exec("UPDATE sms_messages SET "+field+" = ? WHERE sms_id = ?", stored, smsID); err != nil {
			t.Fatal(err)
		}
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
	if _, err := locked.GetMessage(ctx, f.ownerID, smsID); !errors.Is(err, ErrCiphertext) {
		t.Fatalf("wrong keyring decrypted message: %v", err)
	}
	// The dedup path must also fail closed instead of reporting duplicate.
	if _, err := locked.InsertMessage(ctx, f.ownerID, input, f.baseTime); !errors.Is(err, ErrCiphertext) {
		t.Fatalf("wrong keyring dedup check: %v", err)
	}
}

func TestSMSCursorPaginationStability(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	device := f.device(t, "Xiaomi Agent")
	other := f.device(t, "Second Agent")
	ids := make([]string, 0, 5)
	for i := range 5 {
		input := f.messageInput(device.ID, "evt-"+string(rune('a'+i)), "synthetic body "+string(rune('a'+i)))
		input.DeviceAt = f.baseTime.Add(time.Duration(i) * time.Minute)
		if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
		var id string
		if err := f.store.DB.QueryRow(`SELECT sms_id FROM sms_messages
			WHERE remote_event_id = ?`, input.RemoteEventID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	noise := f.messageInput(other.ID, "evt-other", "other device body")
	if _, err := f.store.InsertMessage(ctx, f.ownerID, noise, f.baseTime.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	first, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{DeviceID: device.ID, Limit: 2})
	if err != nil || len(first.Messages) != 2 || first.NextCursor == "" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	if first.Messages[0].ID != ids[4] || first.Messages[1].ID != ids[3] {
		t.Fatalf("first page order: %+v", first.Messages)
	}
	newer := f.messageInput(device.ID, "evt-new", "newer synthetic body")
	if _, err := f.store.InsertMessage(ctx, f.ownerID, newer, f.baseTime.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	second, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{DeviceID: device.ID, Limit: 2, Cursor: first.NextCursor})
	if err != nil || len(second.Messages) != 2 {
		t.Fatalf("second page: %+v, %v", second, err)
	}
	if second.Messages[0].ID != ids[2] || second.Messages[1].ID != ids[1] {
		t.Fatalf("second page shifted by newer record: %+v", second.Messages)
	}
	third, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{DeviceID: device.ID, Limit: 2, Cursor: second.NextCursor})
	if err != nil || len(third.Messages) != 1 || third.Messages[0].ID != ids[0] || third.NextCursor != "" {
		t.Fatalf("third page: %+v, %v", third, err)
	}
	hits, err := f.store.Search(ctx, f.ownerID, SearchOptions{DeviceID: other.ID, Query: "other device"})
	if err != nil || len(hits) != 1 || hits[0].DeviceID != other.ID {
		t.Fatalf("device scoped search: %+v, %v", hits, err)
	}
	if leaked, err := f.store.Search(ctx, "other-owner", SearchOptions{Query: "synthetic"}); err != nil || len(leaked) != 0 {
		t.Fatalf("owner scope leaked into search: %+v, %v", leaked, err)
	}
	if _, err := f.store.ListMessages(ctx, f.ownerID, ListOptions{Cursor: "broken"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid cursor accepted: %v", err)
	}
}

func TestSMSRestartSurvival(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	device := f.device(t, "Xiaomi Agent")
	input := f.messageInput(device.ID, "evt-1", "synthetic inbound body")
	if _, err := f.store.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil {
		t.Fatal(err)
	}
	smsID := f.onlySMSID(t)
	if err := f.store.DB.Close(); err != nil {
		t.Fatal(err)
	}
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
	message, err := revived.GetMessage(ctx, f.ownerID, smsID)
	if err != nil || message.Body != "synthetic inbound body" || message.Direction != DirectionInbound {
		t.Fatalf("message lost after restart: %+v, %v", message, err)
	}
	if status, err := revived.InsertMessage(ctx, f.ownerID, input, f.baseTime); err != nil || status != StatusDuplicate {
		t.Fatalf("dedup state lost after restart: %s, %v", status, err)
	}
}

func TestCommandLifecycleAndIdempotency(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	device := f.device(t, "Xiaomi Agent")
	other := f.device(t, "Second Agent")
	input := f.commandInput(device.ID, "idempotency-key-0001", "synthetic send body")
	command, status, err := f.store.CreateCommand(ctx, f.ownerID, input, f.baseTime)
	if err != nil || status != StatusAccepted || command.Status != StatusQueued {
		t.Fatalf("create command: %s %+v, %v", status, command, err)
	}
	if command.Recipient != "100087" || command.Body != "synthetic send body" {
		t.Fatalf("command content mismatch: %+v", command)
	}
	repeat, status, err := f.store.CreateCommand(ctx, f.ownerID, input, f.baseTime.Add(time.Minute))
	if err != nil || status != StatusDuplicate || repeat.ID != command.ID || repeat.Status != StatusQueued {
		t.Fatalf("idempotent retry: %s %+v, %v", status, repeat, err)
	}
	clash := input
	clash.Body = "different synthetic body"
	if _, _, err := f.store.CreateCommand(ctx, f.ownerID, clash, f.baseTime); !errors.Is(err, ErrConflict) {
		t.Fatalf("same key different content: %v", err)
	}
	wrongDevice := input
	wrongDevice.DeviceID = other.ID
	if _, _, err := f.store.CreateCommand(ctx, f.ownerID, wrongDevice, f.baseTime); !errors.Is(err, ErrConflict) {
		t.Fatalf("same key different device: %v", err)
	}
	if count := f.commandCountForKey(t, input.IdempotencyKey); count != 1 {
		t.Fatalf("command rows for key: %d", count)
	}
	pending, err := f.store.PendingForDevice(ctx, device.ID, 10)
	if err != nil || len(pending) != 1 || pending[0].ID != command.ID {
		t.Fatalf("pending commands: %+v, %v", pending, err)
	}
	if foreign, err := f.store.PendingForDevice(ctx, other.ID, 10); err != nil || len(foreign) != 0 {
		t.Fatalf("command visible to other device: %+v, %v", foreign, err)
	}
	if _, _, err := f.store.CreateCommand(ctx, "other-owner", input, f.baseTime); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner command create: %v", err)
	}
	now := f.baseTime.Add(2 * time.Minute)
	if err := f.store.Claim(ctx, other.ID, command.ID, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-device claim: %v", err)
	}
	if err := f.store.Claim(ctx, device.ID, command.ID, now); err != nil {
		t.Fatal(err)
	}
	claimed, err := f.store.GetCommand(ctx, f.ownerID, command.ID)
	if err != nil || claimed.Status != StatusClaimed || claimed.ClaimedAt == nil {
		t.Fatalf("claim state: %+v, %v", claimed, err)
	}
	if err := f.store.Claim(ctx, device.ID, command.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("idempotent re-claim: %v", err)
	}
	still, err := f.store.GetCommand(ctx, f.ownerID, command.ID)
	if err != nil || !still.ClaimedAt.Equal(claimed.ClaimedAt.UTC()) {
		t.Fatalf("re-claim mutated timestamps: %+v, %v", still, err)
	}
	if err := f.store.MarkSystemSent(ctx, device.ID, command.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkDeliveryReceipt(ctx, device.ID, command.ID, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	resolved, err := f.store.GetCommand(ctx, f.ownerID, command.ID)
	if err != nil || resolved.Status != StatusDeliveryReceipt || resolved.ResolvedAt == nil || resolved.SystemSentAt == nil {
		t.Fatalf("resolved state: %+v, %v", resolved, err)
	}
	// Audit trail records the state machine without any payload content.
	rows, err := f.store.DB.Query(`SELECT action, detail FROM audit_entries
		WHERE action LIKE 'sms.command.%' ORDER BY created_at_ms, entry_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	actions := make([]string, 0)
	for rows.Next() {
		var action, detail string
		if err := rows.Scan(&action, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, command.Recipient) || strings.Contains(detail, command.Body) {
			t.Fatalf("audit detail leaks payload: %s", detail)
		}
		actions = append(actions, action)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		t.Fatal(rowsErr)
	}
	expected := []string{"sms.command.create", "sms.command.claim", "sms.command.system_sent", "sms.command.delivery_receipt"}
	if len(actions) != len(expected) {
		t.Fatalf("audit actions: %v", actions)
	}
	for i, action := range expected {
		if actions[i] != action {
			t.Fatalf("audit actions: %v", actions)
		}
	}
	// Explicit failure and unknown paths remain available for later commands.
	failed, _, err := f.store.CreateCommand(ctx, f.ownerID, f.commandInput(device.ID, "idempotency-key-0002", "body two"), f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.Claim(ctx, device.ID, failed.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkFailed(ctx, device.ID, failed.ID, now); err != nil {
		t.Fatal(err)
	}
	unknown, _, err := f.store.CreateCommand(ctx, f.ownerID, f.commandInput(device.ID, "idempotency-key-0003", "body three"), f.baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.Claim(ctx, device.ID, unknown.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkSystemSent(ctx, device.ID, unknown.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkUnknown(ctx, device.ID, unknown.ID, now); err != nil {
		t.Fatal(err)
	}
	final, err := f.store.GetCommand(ctx, f.ownerID, unknown.ID)
	if err != nil || final.Status != StatusUnknown {
		t.Fatalf("unknown state: %+v, %v", final, err)
	}
}

func TestCommandIllegalTransitions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	device := f.device(t, "Xiaomi Agent")
	now := f.baseTime
	newCommand := func(t *testing.T, key string) Command {
		t.Helper()
		command, _, err := f.store.CreateCommand(ctx, f.ownerID, f.commandInput(device.ID, key, "synthetic body"), now)
		if err != nil {
			t.Fatal(err)
		}
		return command
	}
	// From queued, only claim is legal.
	queued := newCommand(t, "illegal-key-0000001")
	if err := f.store.MarkSystemSent(ctx, device.ID, queued.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("queued to system_sent: %v", err)
	}
	if err := f.store.MarkDeliveryReceipt(ctx, device.ID, queued.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("queued to delivery_receipt: %v", err)
	}
	if err := f.store.MarkFailed(ctx, device.ID, queued.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("queued to failed: %v", err)
	}
	if err := f.store.MarkUnknown(ctx, device.ID, queued.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("queued to unknown: %v", err)
	}
	// From claimed, delivery_receipt must not skip system_sent; unknown is legal.
	claimed := newCommand(t, "illegal-key-0000002")
	if err := f.store.Claim(ctx, device.ID, claimed.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkDeliveryReceipt(ctx, device.ID, claimed.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("claimed to delivery_receipt: %v", err)
	}
	if err := f.store.MarkUnknown(ctx, device.ID, claimed.ID, now); err != nil {
		t.Fatalf("claimed to unknown: %v", err)
	}
	// unknown is terminal and must never re-enqueue (no automatic resend).
	if err := f.store.Claim(ctx, device.ID, claimed.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown reclaimed: %v", err)
	}
	if err := f.store.MarkSystemSent(ctx, device.ID, claimed.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown to system_sent: %v", err)
	}
	if err := f.store.MarkFailed(ctx, device.ID, claimed.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown to failed: %v", err)
	}
	if err := f.store.MarkUnknown(ctx, device.ID, claimed.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown re-marked: %v", err)
	}
	retry, status, err := f.store.CreateCommand(ctx, f.ownerID, f.commandInput(device.ID, "illegal-key-0000002", "synthetic body"), now)
	if err != nil || status != StatusDuplicate || retry.ID != claimed.ID || retry.Status != StatusUnknown {
		t.Fatalf("unknown command re-enqueued: %s %+v, %v", status, retry, err)
	}
	if count := f.commandCountForKey(t, "illegal-key-0000002"); count != 1 {
		t.Fatalf("rows for retried key: %d", count)
	}
	if pending, err := f.store.PendingForDevice(ctx, device.ID, 10); err != nil || len(pending) != 1 {
		t.Fatalf("queued commands after unknown: %+v, %v", pending, err)
	}
	// From delivery_receipt everything is rejected.
	receipt := newCommand(t, "illegal-key-0000003")
	if err := f.store.Claim(ctx, device.ID, receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkSystemSent(ctx, device.ID, receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkDeliveryReceipt(ctx, device.ID, receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkFailed(ctx, device.ID, receipt.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("delivery_receipt to failed: %v", err)
	}
	if err := f.store.Claim(ctx, device.ID, receipt.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("delivery_receipt reclaimed: %v", err)
	}
	// From system_sent a claim is rejected but failure is reported explicitly.
	sent := newCommand(t, "illegal-key-0000004")
	if err := f.store.Claim(ctx, device.ID, sent.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkSystemSent(ctx, device.ID, sent.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Claim(ctx, device.ID, sent.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("system_sent reclaimed: %v", err)
	}
	if err := f.store.MarkFailed(ctx, device.ID, sent.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkUnknown(ctx, device.ID, sent.ID, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("failed re-resolved: %v", err)
	}
	if err := f.store.Claim(ctx, "missing-device", sent.ID, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign device transition: %v", err)
	}
}

func TestCommandRateLimitAndRollback(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	device := f.device(t, "Xiaomi Agent")
	now := f.baseTime
	ids := make([]string, 0, MaxQueuedCommandsPerOwner)
	for i := range MaxQueuedCommandsPerOwner {
		command, _, err := f.store.CreateCommand(ctx, f.ownerID,
			f.commandInput(device.ID, fmt.Sprintf("rate-limit-key-%04d", i), "synthetic body"), now)
		if err != nil {
			t.Fatalf("command %d: %v", i, err)
		}
		ids = append(ids, command.ID)
	}
	if _, _, err := f.store.CreateCommand(ctx, f.ownerID, f.commandInput(device.ID, "rate-limit-key-9999", "synthetic body"), now); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("queued limit bypassed: %v", err)
	}
	// The limit counts queued commands only: claiming frees a slot.
	if err := f.store.Claim(ctx, device.ID, ids[0], now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.store.CreateCommand(ctx, f.ownerID, f.commandInput(device.ID, "rate-limit-key-9998", "synthetic body"), now); err != nil {
		t.Fatalf("limit must count queued commands only: %v", err)
	}
	// A deterministic audit failure must roll the whole create back.
	if err := f.store.Claim(ctx, device.ID, ids[1], now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB.Exec(`CREATE TRIGGER fail_command_create BEFORE INSERT ON audit_entries
		WHEN NEW.action = 'sms.command.create' BEGIN SELECT RAISE(ABORT, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.store.CreateCommand(ctx, f.ownerID, f.commandInput(device.ID, "rate-limit-key-9997", "synthetic body"), now); err == nil {
		t.Fatal("create succeeded despite injected audit failure")
	}
	if count := f.commandCountForKey(t, "rate-limit-key-9997"); count != 0 {
		t.Fatalf("failed create left partial writes: %d", count)
	}
}
