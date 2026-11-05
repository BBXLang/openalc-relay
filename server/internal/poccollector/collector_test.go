package poccollector

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixture(t *testing.T) Batch {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "packages", "protocol", "fixtures", "synthetic-batch.json"))
	if err != nil {
		t.Fatal(err)
	}
	var batch Batch
	if err := json.Unmarshal(data, &batch); err != nil {
		t.Fatal(err)
	}
	return batch
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "collector.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCommitAndRestartDeduplication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.db")
	batch := fixture(t)
	for _, expected := range []string{"persisted", "duplicate"} {
		store, err := Open(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := store.Persist(context.Background(), batch)
		if err != nil || receipt.Acknowledgements[0].Status != expected {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
		count, err := store.Count(context.Background())
		if err != nil || count != 1 {
			t.Fatalf("count=%d err=%v", count, err)
		}
		store.Close()
	}
}

func TestPayloadConflictAndInstallationScope(t *testing.T) {
	store := testStore(t)
	batch := fixture(t)
	if _, err := store.Persist(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	batch.Events[0].ReceivedAtMS++
	receipt, err := store.Persist(context.Background(), batch)
	if err != nil || receipt.Acknowledgements[0].Code != "event_id_conflict" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	batch.InstallationID = "00000000-0000-4000-8000-000000000003"
	if _, err := store.Persist(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if count, _ := store.Count(context.Background()); count != 2 {
		t.Fatalf("installation scope lost: count=%d", count)
	}
}

func TestConcurrentDuplicateUploads(t *testing.T) {
	store := testStore(t)
	batch := fixture(t)
	var group sync.WaitGroup
	for range 100 {
		group.Go(func() {
			if _, err := store.Persist(context.Background(), batch); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if count, _ := store.Count(context.Background()); count != 1 {
		t.Fatalf("deduplication failed: count=%d", count)
	}
}

func TestFailedTransactionHasNoACKOrPartialWrites(t *testing.T) {
	store := testStore(t)
	batch := fixture(t)
	batch.Events = append(batch.Events, Event{EventID: "00000000-0000-4000-8000-000000000003", FixtureID: "00000000-0000-4000-8000-000000000003", Source: "injected", ReceivedAtMS: 1793577600000})
	_, err := store.db.Exec(`CREATE TRIGGER fail_second BEFORE INSERT ON synthetic_events WHEN NEW.event_id = '00000000-0000-4000-8000-000000000003' BEGIN SELECT RAISE(ABORT, 'injected'); END`)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.Persist(context.Background(), batch)
	if err == nil || len(receipt.Acknowledgements) != 0 {
		t.Fatalf("uncommitted records were acknowledged: %+v", receipt)
	}
	if count, _ := store.Count(context.Background()); count != 0 {
		t.Fatalf("partial commit: count=%d", count)
	}
}

func TestACKLostAfterCommit(t *testing.T) {
	store := testStore(t)
	server := httptest.NewServer(NewHandler(store, true))
	defer server.Close()
	data, _ := json.Marshal(fixture(t))
	response, err := http.Post(server.URL+"/poc/v1/events", "application/json", bytes.NewReader(data))
	if err == nil {
		response.Body.Close()
		t.Fatal("first response should be dropped")
	}
	if count, _ := store.Count(context.Background()); count != 1 {
		t.Fatalf("event not committed before drop: count=%d", count)
	}
	response, err = http.Post(server.URL+"/poc/v1/events", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var receipt Receipt
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil || receipt.Acknowledgements[0].Status != "duplicate" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
}

func TestPrivacyAndLoopbackBoundary(t *testing.T) {
	store := testStore(t)
	handler := NewHandler(store, false)
	data, _ := json.Marshal(fixture(t))
	for _, test := range []struct {
		name, peer, origin, contentType, body string
		status                                int
	}{
		{"public peer", "192.0.2.1:1234", "", "application/json", string(data), 403},
		{"browser origin", "127.0.0.1:1234", "https://example.invalid", "application/json", string(data), 403},
		{"wrong media type", "127.0.0.1:1234", "", "text/plain", string(data), 415},
		{"unknown body field", "127.0.0.1:1234", "", "application/json", strings.Replace(string(data), `"source":"injected"`, `"source":"injected","body":"SYNTHETIC-PRIVATE-CANARY"`, 1), 400},
		{"trailing document", "127.0.0.1:1234", "", "application/json", string(data) + "{}", 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "/poc/v1/events", strings.NewReader(test.body))
			request.RemoteAddr = test.peer
			request.Host = "127.0.0.1:8787"
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Content-Type", test.contentType)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status || strings.Contains(recorder.Body.String(), "SYNTHETIC-PRIVATE-CANARY") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
	if count, _ := store.Count(context.Background()); count != 0 {
		t.Fatalf("invalid request persisted: count=%d", count)
	}
}

func TestStorageFailureReturnsUnavailable(t *testing.T) {
	store := testStore(t)
	store.Close()
	data, _ := json.Marshal(fixture(t))
	request := httptest.NewRequest("POST", "/poc/v1/events", bytes.NewReader(data))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Host = "127.0.0.1:8787"
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	NewHandler(store, false).ServeHTTP(recorder, request)
	body, _ := io.ReadAll(recorder.Result().Body)
	if recorder.Code != 503 || strings.Contains(string(body), "persisted") {
		t.Fatalf("status=%d body=%s", recorder.Code, body)
	}
}
