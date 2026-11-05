package poccollector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestScopedPaginationSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "collector.db")
	store, err := Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	batch := fixture(t)
	for i := 1; i <= 57; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		batch.Events[0].EventID, batch.Events[0].FixtureID = id, id
		if _, err := store.Persist(ctx, batch); err != nil {
			t.Fatal(err)
		}
	}
	other := batch
	other.InstallationID = "00000000-0000-4000-8000-000000000099"
	if _, err := store.Persist(ctx, other); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := NewHandler(store, false)
	after, seen := "", map[string]bool{}
	for _, size := range []int{25, 25, 7} {
		target := "/poc/v1/events?installation_id=" + batch.InstallationID
		if after != "" {
			target += "&after=" + after
		}
		r := localGET(target)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		var page EventPage
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || w.Code != 200 || len(page.Events) != size || page.Total != 57 || page.InstallationID != batch.InstallationID {
			t.Fatalf("code=%d page=%+v err=%v", w.Code, page, err)
		}
		if strings.Contains(w.Body.String(), "payload_checksum") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private storage fields or caching exposed")
		}
		for _, event := range page.Events {
			if seen[event.EventID] || event.EventID <= after {
				t.Fatal("duplicate or unstable order")
			}
			seen[event.EventID] = true
		}
		if size == 25 {
			if page.NextAfter == nil {
				t.Fatal("missing cursor")
			}
			after = *page.NextAfter
		} else if page.NextAfter != nil {
			t.Fatal("unexpected next page")
		}
	}
	if len(seen) != 57 {
		t.Fatal("lost records")
	}
	empty, err := store.List(ctx, "00000000-0000-4000-8000-000000000098", "", 25)
	if err != nil || empty.Total != 0 || len(empty.Events) != 0 || empty.Events == nil || empty.NextAfter != nil {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
}

func localGET(target string) *http.Request {
	r := httptest.NewRequest("GET", "http://127.0.0.1:8787"+target, nil)
	r.RemoteAddr = "127.0.0.1:1234"
	return r
}

func TestReadValidationAndLocalBrowserBoundary(t *testing.T) {
	store := testStore(t)
	handler := NewHandler(store, false)
	id := fixture(t).InstallationID
	for _, query := range []string{"", "installation_id=bad", "installation_id=" + id + "&installation_id=" + id, "installation_id=" + id + "&after=", "installation_id=" + id + "&limit=0", "installation_id=" + id + "&limit=51", "installation_id=" + id + "&limit=x", "installation_id=" + id + "&body=x", "installation_id=" + id + "&bad=%xx"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, localGET("/poc/v1/events?"+query))
		if w.Code != 400 {
			t.Fatalf("query=%s code=%d", query, w.Code)
		}
	}
	for _, test := range []struct {
		host, peer, origin, site string
		status                   int
	}{
		{"127.0.0.1:8787", "127.0.0.1:1234", "", "same-origin", 200},
		{"127.0.0.1:8787", "127.0.0.1:1234", "http://127.0.0.1:8787", "same-origin", 200},
		{"attacker.invalid:8787", "127.0.0.1:1234", "", "", 403},
		{"127.0.0.1:8787", "192.0.2.1:1234", "", "", 403},
		{"127.0.0.1:8787", "127.0.0.1:1234", "https://attacker.invalid", "", 403},
		{"127.0.0.1:8787", "127.0.0.1:1234", "http://127.0.0.1:8788", "same-site", 403},
		{"127.0.0.1:8787", "127.0.0.1:1234", "", "cross-site", 403},
	} {
		r := localGET("/poc/v1/events?installation_id=" + id)
		r.Host, r.RemoteAddr = test.host, test.peer
		r.Header.Set("Origin", test.origin)
		r.Header.Set("Sec-Fetch-Site", test.site)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("test=%+v code=%d", test, w.Code)
		}
	}
	// Even same-origin browser POST is forbidden: the Web verification page is read-only.
	r := localGET("/poc/v1/events")
	r.Method = "POST"
	r.Header.Set("Origin", "http://127.0.0.1:8787")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("browser write allowed")
	}
	store.Close()
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, localGET("/poc/v1/events?installation_id="+id))
	if w.Code != 503 {
		t.Fatal("storage error hidden as success")
	}
}

func TestWebAssetsAreExplicitAndRestricted(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html><title>synthetic</title>")},
		"assets/main.js":  {Data: []byte("export {}")},
		"assets/main.css": {Data: []byte("body {}")},
		"secret.txt":      {Data: []byte("PRIVATE-CANARY")},
	}
	store := testStore(t)
	handler := NewWebHandler(store, false, assets)
	for target, status := range map[string]int{"/": 200, "/assets/main.js": 200, "/assets/main.css": 200, "/secret.txt": 404, "/assets/../secret.txt": 404, "/assets/": 404, "/assets/.env": 404, "/assets/missing.js": 404, "/missing": 404} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, localGET(target))
		if w.Code != status || strings.Contains(w.Body.String(), "PRIVATE-CANARY") {
			t.Fatalf("target=%s code=%d", target, w.Code)
		}
		if status == 200 && (w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Content-Type-Options") != "nosniff") {
			t.Fatal("missing browser policy")
		}
	}
	r := localGET("/")
	r.Method = "HEAD"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("HEAD failed")
	}
	w = httptest.NewRecorder()
	NewHandler(store, false).ServeHTTP(w, localGET("/"))
	if w.Code != 404 {
		t.Fatal("Web enabled implicitly")
	}
}

// Contract consistency check for the implemented PoC read surface (not a full schema validator).
func TestPoCReadOpenAPIContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "packages", "openapi", "poc-collector.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Paths map[string]map[string]struct{ Responses map[string]json.RawMessage }
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	read, ok := contract.Paths["/poc/v1/events"]["get"]
	if !ok || len(read.Responses) != 5 {
		t.Fatal("missing read contract responses")
	}
	for _, code := range []string{"200", "400", "403", "405", "503"} {
		if _, ok := read.Responses[code]; !ok {
			t.Fatal("missing status " + code)
		}
	}
}
