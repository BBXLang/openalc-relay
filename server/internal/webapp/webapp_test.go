package webapp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildWebDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"index.html":         "<!doctype html><title>Relay</title><div id=app></div>",
		"assets/app.1a2.js":  "console.log('relay')",
		"assets/app.3b4.css": "body{}",
		"secret.txt":         "must not be served",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestWebappServing(t *testing.T) {
	handler, err := New(buildWebDir(t))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		method       string
		target       string
		wantStatus   int
		wantCache    string
		wantType     string
		wantContains string
	}{
		{name: "root serves index", method: http.MethodGet, target: "/", wantStatus: 200,
			wantCache: "no-cache", wantType: "text/html; charset=utf-8", wantContains: "<!doctype html>"},
		{name: "extensionless fallback", method: http.MethodGet, target: "/accounts/inbox", wantStatus: 200,
			wantCache: "no-cache", wantType: "text/html; charset=utf-8"},
		{name: "asset long cache", method: http.MethodGet, target: "/assets/app.1a2.js", wantStatus: 200,
			wantCache: "public, max-age=31536000, immutable", wantType: "text/javascript; charset=utf-8"},
		{name: "asset css", method: http.MethodGet, target: "/assets/app.3b4.css", wantStatus: 200,
			wantCache: "public, max-age=31536000, immutable", wantType: "text/css; charset=utf-8"},
		{name: "missing asset", method: http.MethodGet, target: "/assets/nope.9z9.js", wantStatus: 404},
		{name: "build file outside assets", method: http.MethodGet, target: "/secret.txt", wantStatus: 404},
		{name: "traversal refused", method: http.MethodGet, target: "/../relay.db", wantStatus: 404},
		{name: "index.html alias refused", method: http.MethodGet, target: "/index.html", wantStatus: 404},
		{name: "head works", method: http.MethodHead, target: "/", wantStatus: 200, wantCache: "no-cache"},
		{name: "post rejected", method: http.MethodPost, target: "/", wantStatus: 405},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(test.method, "http://127.0.0.1"+test.target, nil)
			// httptest.NewRequest would clean "/../relay.db"; build the URL
			// object directly so the handler receives the raw path it would
			// see from a hostile client before any server-level cleaning.
			r.URL.Path = test.target
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != test.wantStatus {
				t.Fatalf("got %d, want %d (%s)", w.Code, test.wantStatus, w.Body.String())
			}
			if test.wantCache != "" {
				if got := w.Header().Get("Cache-Control"); got != test.wantCache {
					t.Fatalf("Cache-Control %q, want %q", got, test.wantCache)
				}
			}
			if test.wantType != "" {
				if got := w.Header().Get("Content-Type"); got != test.wantType {
					t.Fatalf("Content-Type %q, want %q", got, test.wantType)
				}
			}
			if test.wantContains != "" && !strings.Contains(w.Body.String(), test.wantContains) {
				t.Fatalf("body %q does not contain %q", w.Body.String(), test.wantContains)
			}
			if test.wantStatus == 200 {
				// Security headers must be on every successful response, and
				// the CSP must never allow foreign origins.
				for _, header := range []string{"Content-Security-Policy", "Referrer-Policy", "Cross-Origin-Resource-Policy"} {
					if w.Header().Get(header) == "" {
						t.Fatalf("missing security header %s", header)
					}
				}
				if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
					t.Fatal("unexpected CSP")
				}
			}
		})
	}
}

func TestWebappNewRequiresIndex(t *testing.T) {
	if _, err := New(t.TempDir()); err == nil {
		t.Fatal("directory without index.html must be rejected")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "index.html"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir); err == nil {
		t.Fatal("non-regular index.html must be rejected")
	}
}
