package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	for _, test := range []struct {
		name, method, path string
		databaseError      error
		wantStatus         int
		wantBody           string
	}{
		{"liveness independent of database", "GET", "/healthz", errors.New("private database detail"), 200, `{"status":"ok"}`},
		{"ready", "GET", "/readyz", nil, 200, `{"status":"ok"}`},
		{"not ready redacts error", "GET", "/readyz", errors.New("private database detail"), 503, `{"status":"unavailable"}`},
		{"head", "HEAD", "/healthz", nil, 200, ""},
		{"no business API", "POST", "/v1/commands", nil, 404, `{"type":"about:blank","title":"Not Found","status":404}`},
		{"read only", "POST", "/healthz", nil, 405, `{"type":"about:blank","title":"Method Not Allowed","status":405}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("readiness check has no deadline")
				}
				return test.databaseError
			})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if response.Code != test.wantStatus || strings.TrimSpace(response.Body.String()) != test.wantBody {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("health response must not be cached")
			}
			if response.Code == http.StatusMethodNotAllowed && response.Header().Get("Allow") != "GET, HEAD" {
				t.Fatal("missing Allow header")
			}
		})
	}
}
