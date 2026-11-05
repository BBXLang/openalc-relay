package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
)

func businessTestHandler(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := identity.Bootstrap(ctx, db, []byte("local owner passphrase")); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "keys.json")
	if err := secrets.CreateFile(keyPath); err != nil {
		t.Fatal(err)
	}
	keys, err := secrets.LoadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewBusinessHandler(db.PingContext, db, keys, "https://relay.test")
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func sessionRequest(method string, body string) *http.Request {
	r := httptest.NewRequest(method, "https://relay.test/v1/session", strings.NewReader(body))
	r.Host = "relay.test"
	if method != http.MethodGet {
		r.Header.Set("Origin", "https://relay.test")
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func TestOwnerSessionLoginLogout(t *testing.T) {
	handler := businessTestHandler(t)
	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, sessionRequest(http.MethodGet, ""))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous session read: %d", unauthenticated.Code)
	}
	wrong := httptest.NewRecorder()
	handler.ServeHTTP(wrong, sessionRequest(http.MethodPost, `{"password":"wrong password here"}`))
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password accepted: %d", wrong.Code)
	}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, sessionRequest(http.MethodPost, `{"password":"local owner passphrase"}`))
	if login.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", login.Code, login.Body.String())
	}
	var response struct {
		OwnerID   string `json:"owner_id"`
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &response); err != nil || response.OwnerID == "" || response.CSRFToken == "" {
		t.Fatalf("invalid login response: %v", err)
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName ||
		!cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie security attributes missing")
	}
	read := sessionRequest(http.MethodGet, "")
	read.AddCookie(cookies[0])
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, read)
	if got.Code != http.StatusOK {
		t.Fatalf("authenticated session read failed: %d", got.Code)
	}
	logout := sessionRequest(http.MethodDelete, "")
	logout.AddCookie(cookies[0])
	withoutCSRF := httptest.NewRecorder()
	handler.ServeHTTP(withoutCSRF, logout)
	if withoutCSRF.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF accepted: %d", withoutCSRF.Code)
	}
	logout.Header.Set("X-CSRF-Token", response.CSRFToken)
	revoked := httptest.NewRecorder()
	handler.ServeHTTP(revoked, logout)
	if revoked.Code != http.StatusNoContent {
		t.Fatalf("logout failed: %d", revoked.Code)
	}
	after := sessionRequest(http.MethodGet, "")
	after.AddCookie(cookies[0])
	afterResponse := httptest.NewRecorder()
	handler.ServeHTTP(afterResponse, after)
	if afterResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session accepted: %d", afterResponse.Code)
	}
}

func TestOwnerSessionRejectsCrossOriginAndUnknownFields(t *testing.T) {
	handler := businessTestHandler(t)
	request := sessionRequest(http.MethodPost, `{"password":"local owner passphrase"}`)
	request.Header.Set("Origin", "https://other.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin login accepted: %d", response.Code)
	}
	request = sessionRequest(http.MethodPost, `{"password":"local owner passphrase","owner_id":"attacker"}`)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field accepted: %d", response.Code)
	}
	request = sessionRequest(http.MethodPost, `{"password":"local owner passphrase"}`)
	request.Host = "other.test"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("wrong Host accepted: %d", response.Code)
	}
}

func TestOwnerLoginRateLimit(t *testing.T) {
	handler := businessTestHandler(t)
	for range 10 {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, sessionRequest(http.MethodPost, `{"password":"wrong password here"}`))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("unexpected login response: %d", response.Code)
		}
	}
	limited := httptest.NewRecorder()
	handler.ServeHTTP(limited, sessionRequest(http.MethodPost, `{"password":"local owner passphrase"}`))
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Retry-After") != "60" {
		t.Fatalf("login rate limit failed: %d", limited.Code)
	}
}
