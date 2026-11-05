package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The mail account surface is the first formal data slice: credentials are
// write-only and must never appear in any response.
func loginOwner(t *testing.T, handler http.Handler) (*http.Cookie, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, sessionRequest(http.MethodPost, `{"password":"local owner passphrase"}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("owner login: %d %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.CSRFToken == "" {
		t.Fatalf("session response: %v", err)
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie, body.CSRFToken
		}
	}
	t.Fatal("session cookie missing")
	return nil, ""
}

func mailRequest(method, path, body string, cookie *http.Cookie, csrf string) *http.Request {
	r := httptest.NewRequest(method, "https://relay.test"+path, nil)
	r.Host = "relay.test"
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if method != http.MethodGet {
		r.Header.Set("Origin", "https://relay.test")
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	if body != "" {
		r.Body = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)).Body
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

const mailAccountPayload = `{"address":"user@example.test","imap_host":"imap.example.test",
	"imap_port":993,"smtp_host":"smtp.example.test","smtp_port":465,
	"tls_mode":"implicit_tls","username":"user@example.test","password":"synthetic-app-password"}`

func TestMailAccountsRequireSessionAndCSRF(t *testing.T) {
	handler := businessTestHandler(t)
	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, mailRequest(http.MethodGet, "/v1/mail-accounts", "", nil, ""))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list: %d", anonymous.Code)
	}
	cookie, csrf := loginOwner(t, handler)
	missingCSRF := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRF, mailRequest(http.MethodPost, "/v1/mail-accounts", mailAccountPayload, cookie, ""))
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF accepted: %d", missingCSRF.Code)
	}
	crossOrigin := httptest.NewRecorder()
	request := mailRequest(http.MethodPost, "/v1/mail-accounts", mailAccountPayload, cookie, csrf)
	request.Header.Set("Origin", "https://evil.example")
	handler.ServeHTTP(crossOrigin, request)
	if crossOrigin.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation accepted: %d", crossOrigin.Code)
	}
}

func TestMailAccountLifecycleOverHTTP(t *testing.T) {
	handler := businessTestHandler(t)
	cookie, csrf := loginOwner(t, handler)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, mailRequest(http.MethodPost, "/v1/mail-accounts", mailAccountPayload, cookie, csrf))
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	if strings.Contains(created.Body.String(), "synthetic-app-password") ||
		strings.Contains(strings.ToLower(created.Body.String()), "password") ||
		strings.Contains(strings.ToLower(created.Body.String()), "credential") {
		t.Fatalf("response leaks credential material: %s", created.Body.String())
	}
	var account struct {
		ID string `json:"account_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &account); err != nil || account.ID == "" {
		t.Fatalf("account response: %v", err)
	}
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, mailRequest(http.MethodPost, "/v1/mail-accounts",
		strings.Replace(mailAccountPayload, `"implicit_tls"`, `"plain"`, 1), cookie, csrf))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid account accepted: %d", invalid.Code)
	}
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, mailRequest(http.MethodGet, "/v1/mail-accounts", "", cookie, ""))
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), account.ID) {
		t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
	}
	if strings.Contains(listed.Body.String(), "synthetic-app-password") {
		t.Fatal("list leaks credential material")
	}
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, mailRequest(http.MethodDelete, "/v1/mail-accounts/00000000-0000-4000-8000-000000000000", "", cookie, csrf))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown delete: %d", unknown.Code)
	}
	deleted := httptest.NewRecorder()
	handler.ServeHTTP(deleted, mailRequest(http.MethodDelete, "/v1/mail-accounts/"+account.ID, "", cookie, csrf))
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", deleted.Code, deleted.Body.String())
	}
	empty := httptest.NewRecorder()
	handler.ServeHTTP(empty, mailRequest(http.MethodGet, "/v1/mail-accounts", "", cookie, ""))
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"accounts":[]`) {
		t.Fatalf("list after delete: %d %s", empty.Code, empty.Body.String())
	}
}
