package httpapi

import (
	"context"
	"crypto/subtle"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/mailadapter"
	"github.com/OpenALC/relay/server/internal/secrets"
)

const sessionCookieName = "__Host-relay-session"

type businessHandler struct {
	health      http.Handler
	db          *sql.DB
	keys        *secrets.Keyring
	origin      string
	host        string
	loginMu     sync.Mutex
	loginWindow time.Time
	loginCount  int
	loginSlots  chan struct{}
	// mailPolicy restricts the IMAP endpoints the manual sync and
	// connection-test slices may dial. The zero value is the strict
	// production default (whitelisted ports on public hosts only); tests
	// inject the administrator-style allowlist that admits their in-process
	// 127.0.0.1 endpoints. mailRootCAs swaps the trusted root store for
	// private-CA mail hosts (nil keeps the system roots). Neither field is
	// used by any other surface.
	mailPolicy  mailadapter.EndpointPolicy
	mailRootCAs *x509.CertPool
}

// NewBusinessHandler exposes only the implemented owner session endpoint.
// The owner and keyring must already be initialized by the local admin command.
func NewBusinessHandler(checkDatabase func(context.Context) error, db *sql.DB, keys *secrets.Keyring, publicOrigin string) (http.Handler, error) {
	return NewBusinessHandlerWithMailSync(checkDatabase, db, keys, publicOrigin, mailadapter.EndpointPolicy{}, nil)
}

// NewBusinessHandlerWithMailSync is NewBusinessHandler with an explicit
// endpoint policy and root-store override for the manual mail sync slices.
// Production callers use NewBusinessHandler, whose strict default denies
// private and loopback mail targets; only a deployment that deliberately
// passes an allowlist policy (and, for private CAs, a root store) widens it.
func NewBusinessHandlerWithMailSync(checkDatabase func(context.Context) error, db *sql.DB, keys *secrets.Keyring, publicOrigin string, policy mailadapter.EndpointPolicy, rootCAs *x509.CertPool) (http.Handler, error) {
	parsed, err := url.Parse(publicOrigin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || keys == nil {
		return nil, errors.New("business origin or encryption keys invalid")
	}
	return &businessHandler{health: NewHandler(checkDatabase), db: db, keys: keys,
		origin: "https://" + parsed.Host, host: parsed.Host,
		loginSlots: make(chan struct{}, 2), mailPolicy: policy, mailRootCAs: rootCAs}, nil
}

func (h *businessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
		h.health.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Host != h.host || (r.URL.RawQuery != "" && !readQueryAllowed(r.URL.Path)) {
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if r.URL.Path != "/v1/session" {
		if h.serveMailSync(w, r) {
			return
		}
		if h.serveMail(w, r) {
			return
		}
		if h.serveMailbox(w, r) {
			return
		}
		if h.serveSMS(w, r) {
			return
		}
		h.serveSynthetic(w, r)
		return
	}
	switch r.Method {
	case http.MethodPost:
		h.login(w, r)
	case http.MethodGet:
		h.getSession(w, r)
	case http.MethodDelete:
		h.logout(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writeBusinessProblem(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func (h *businessHandler) login(w http.ResponseWriter, r *http.Request) {
	if !h.validOrigin(r) {
		writeBusinessProblem(w, http.StatusForbidden, "origin_rejected")
		return
	}
	if !h.allowLogin(time.Now()) {
		w.Header().Set("Retry-After", "60")
		writeBusinessProblem(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	select {
	case h.loginSlots <- struct{}{}:
		defer func() { <-h.loginSlots }()
	default:
		w.Header().Set("Retry-After", "1")
		writeBusinessProblem(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeBusinessProblem(w, http.StatusUnsupportedMediaType, "json_required")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	var input struct {
		Password string `json:"password"`
	}
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
		return
	}
	ownerID, ok, err := identity.VerifyPassword(r.Context(), h.db, []byte(input.Password))
	if err != nil {
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
		return
	}
	if !ok {
		writeBusinessProblem(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	now := time.Now().UTC()
	token, expires, err := identity.CreateSession(r.Context(), h.db, ownerID, now)
	if err != nil {
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
		return
	}
	csrf, err := h.keys.CSRFToken(token)
	if err != nil {
		writeBusinessProblem(w, http.StatusServiceUnavailable, "key_unavailable")
		return
	}
	h.setSessionCookie(w, token, expires)
	writeSession(w, ownerID, expires, csrf)
}

func (h *businessHandler) getSession(w http.ResponseWriter, r *http.Request) {
	ownerID, token, expires, ok := h.currentSession(w, r)
	if !ok {
		return
	}
	csrf, err := h.keys.CSRFToken(token)
	if err != nil {
		writeBusinessProblem(w, http.StatusServiceUnavailable, "key_unavailable")
		return
	}
	writeSession(w, ownerID, expires, csrf)
}

func (h *businessHandler) logout(w http.ResponseWriter, r *http.Request) {
	if !h.validOrigin(r) {
		writeBusinessProblem(w, http.StatusForbidden, "origin_rejected")
		return
	}
	_, token, _, ok := h.currentSession(w, r)
	if !ok {
		return
	}
	expected, err := h.keys.CSRFToken(token)
	if err != nil {
		writeBusinessProblem(w, http.StatusServiceUnavailable, "key_unavailable")
		return
	}
	provided := r.Header.Get("X-CSRF-Token")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeBusinessProblem(w, http.StatusForbidden, "csrf_rejected")
		return
	}
	if err := identity.RevokeSession(r.Context(), h.db, token, time.Now()); err != nil {
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (h *businessHandler) currentSession(w http.ResponseWriter, r *http.Request) (string, string, time.Time, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeBusinessProblem(w, http.StatusUnauthorized, "session_required")
		return "", "", time.Time{}, false
	}
	ownerID, expires, ok, err := identity.LookupSession(r.Context(), h.db, cookie.Value, time.Now())
	if err != nil {
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
		return "", "", time.Time{}, false
	}
	if !ok {
		writeBusinessProblem(w, http.StatusUnauthorized, "session_required")
		return "", "", time.Time{}, false
	}
	return ownerID, cookie.Value, expires, true
}

func (h *businessHandler) validOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") == h.origin && r.Header.Get("Sec-Fetch-Site") != "cross-site"
}

func (h *businessHandler) allowLogin(now time.Time) bool {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	if h.loginWindow.IsZero() || now.Sub(h.loginWindow) >= time.Minute {
		h.loginWindow = now
		h.loginCount = 0
	}
	if h.loginCount >= 10 {
		return false
	}
	h.loginCount++
	return true
}

func (h *businessHandler) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/",
		Expires: expires, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func writeSession(w http.ResponseWriter, ownerID string, expires time.Time, csrf string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"owner_id": ownerID, "expires_at": expires.Format(time.RFC3339), "csrf_token": csrf,
	})
}

func writeBusinessProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": http.StatusText(status), "status": status, "code": code,
	})
}
