package synthetic

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

func testStore(t *testing.T) (Store, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "relay.db")
	db, err := sqlite.Open(context.Background(), path)
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
	return Store{DB: db}, ownerID, path
}

func pairedDevice(t *testing.T, store Store, ownerID string, now time.Time) Device {
	t.Helper()
	ctx := context.Background()
	pair, err := store.CreatePairing(ctx, ownerID, now)
	if err != nil {
		t.Fatal(err)
	}
	request, err := store.RequestPairing(ctx, pair.ID, "HarmonyOS emulator", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApprovePairing(ctx, ownerID, pair.ID, request.ID, request.ComparisonCode, now); err != nil {
		t.Fatal(err)
	}
	device, err := store.ExchangePairing(ctx, pair.ID, request.ID, request.ClaimToken, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExchangePairing(ctx, pair.ID, request.ID, request.ClaimToken, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("repeated exchange: %v", err)
	}
	return device
}

func TestPairingAndDurableTwoWaySync(t *testing.T) {
	store, ownerID, path := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	pair, err := store.CreatePairing(ctx, ownerID, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPairing(ctx, "other-owner", pair.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner pairing read: %v", err)
	}
	request, err := store.RequestPairing(ctx, pair.ID, "HarmonyOS emulator", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RequestPairing(ctx, pair.ID, "racing emulator", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("second applicant accepted: %v", err)
	}
	if _, err := store.ExchangePairing(ctx, pair.ID, request.ID, request.ClaimToken, now); !errors.Is(err, ErrDenied) {
		t.Fatalf("exchange before approval: %v", err)
	}
	if err := store.ApprovePairing(ctx, ownerID, pair.ID, request.ID, "000000", now); err == nil {
		t.Fatal("wrong comparison code accepted")
	}
	if err := store.ApprovePairing(ctx, ownerID, pair.ID, request.ID, request.ComparisonCode, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExchangePairing(ctx, pair.ID, request.ID, "pr_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now); !errors.Is(err, ErrDenied) {
		t.Fatalf("forged claim accepted: %v", err)
	}
	device, err := store.ExchangePairing(ctx, pair.ID, request.ID, request.ClaimToken, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePing(ctx, "other-owner", device.ID, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner ping accepted: %v", err)
	}
	ping, err := store.CreatePing(ctx, ownerID, device.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Sync(ctx, device.DeviceToken, eventID, "", now)
	if err != nil || result.EventStatus != "accepted" || len(result.Pings) != 1 || result.Pings[0].ID != ping.ID {
		t.Fatalf("initial two-way sync: %+v, %v", result, err)
	}
	result, err = store.Sync(ctx, device.DeviceToken, eventID, ping.ID, now.Add(time.Second))
	if err != nil || result.EventStatus != "duplicate" || len(result.Pings) != 0 {
		t.Fatalf("retry and acknowledgement: %+v, %v", result, err)
	}
	storedPing, err := store.GetPing(ctx, ownerID, ping.ID)
	if err != nil || storedPing.AcknowledgedAt == nil {
		t.Fatalf("ping acknowledgement missing: %v", err)
	}
	if _, err := store.GetPing(ctx, "other-owner", ping.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner ping read: %v", err)
	}
	if err := store.DB.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	store.DB = reopened
	events, err := store.ListEvents(ctx, ownerID)
	if err != nil || len(events) != 1 || events[0].ClientEventID != eventID {
		t.Fatalf("event missing after reopen: %+v, %v", events, err)
	}
	storedPing, err = store.GetPing(ctx, ownerID, ping.ID)
	if err != nil || storedPing.AcknowledgedAt == nil {
		t.Fatalf("ping lost after reopen: %v", err)
	}
	if err := store.RevokeDevice(ctx, "other-owner", device.ID, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner revoke: %v", err)
	}
	if err := store.RevokeDevice(ctx, ownerID, device.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Sync(ctx, device.DeviceToken, "", "", now); !errors.Is(err, ErrDenied) {
		t.Fatalf("revoked device synced: %v", err)
	}
}

func TestSyncRollbackAndConcurrentDeduplication(t *testing.T) {
	store, ownerID, _ := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	device := pairedDevice(t, store, ownerID, now)
	eventID, _ := randomID()
	unknownPingID, _ := randomID()
	if _, err := store.Sync(ctx, device.DeviceToken, eventID, unknownPingID, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid ping acknowledgement: %v", err)
	}
	events, err := store.ListEvents(ctx, ownerID)
	if err != nil || len(events) != 0 {
		t.Fatalf("event committed before failed acknowledgement: %v, %v", events, err)
	}
	var group sync.WaitGroup
	statuses := make(chan string, 12)
	errorsSeen := make(chan error, 12)
	for range 12 {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := store.Sync(ctx, device.DeviceToken, eventID, "", now)
			statuses <- result.EventStatus
			errorsSeen <- err
		}()
	}
	group.Wait()
	close(statuses)
	close(errorsSeen)
	accepted, duplicate := 0, 0
	for status := range statuses {
		switch status {
		case "accepted":
			accepted++
		case "duplicate":
			duplicate++
		default:
			t.Fatalf("unexpected status: %s", status)
		}
	}
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
	if accepted != 1 || duplicate != 11 {
		t.Fatalf("deduplication: accepted=%d duplicate=%d", accepted, duplicate)
	}
	events, err = store.ListEvents(ctx, ownerID)
	if err != nil || len(events) != 1 {
		t.Fatalf("deduplicated event count: %d, %v", len(events), err)
	}
}

func TestPairingExpiryAndAttemptLimit(t *testing.T) {
	store, ownerID, _ := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	pair, err := store.CreatePairing(ctx, ownerID, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RequestPairing(ctx, pair.ID, "HarmonyOS emulator", now.Add(PairingLifetime)); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired invitation accepted: %v", err)
	}
	pair, err = store.CreatePairing(ctx, ownerID, now)
	if err != nil {
		t.Fatal(err)
	}
	request, err := store.RequestPairing(ctx, pair.ID, "HarmonyOS emulator", now)
	if err != nil {
		t.Fatal(err)
	}
	wrong := "000000"
	if wrong == request.ComparisonCode {
		wrong = "111111"
	}
	for range 3 {
		if err := store.ApprovePairing(ctx, ownerID, pair.ID, request.ID, wrong, now); !errors.Is(err, ErrCode) {
			t.Fatalf("wrong code result: %v", err)
		}
	}
	if err := store.ApprovePairing(ctx, ownerID, pair.ID, request.ID, request.ComparisonCode, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("attempt limit bypassed: %v", err)
	}
}
