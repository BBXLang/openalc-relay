package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/OpenALC/relay/server/internal/synthetic"
)

func (h *businessHandler) serveSynthetic(w http.ResponseWriter, r *http.Request) {
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	store := synthetic.Store{DB: h.db}
	switch {
	case r.URL.Path == "/v1/pairings":
		if !methodIs(w, r, http.MethodPost) {
			return
		}
		ownerID, ok := h.requireOwner(w, r, true)
		if !ok {
			return
		}
		if !emptyBody(w, r) {
			return
		}
		pairing, err := store.CreatePairing(r.Context(), ownerID, time.Now())
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, pairing)
	case len(path) == 3 && path[0] == "v1" && path[1] == "pairings":
		if !methodIs(w, r, http.MethodGet) {
			return
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return
		}
		pairing, err := store.GetPairing(r.Context(), ownerID, path[2])
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, pairing)
	case len(path) == 4 && path[0] == "v1" && path[1] == "pairings" && path[3] == "requests":
		if !methodIs(w, r, http.MethodPost) {
			return
		}
		var input struct {
			DeviceLabel string `json:"device_label"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		request, err := store.RequestPairing(r.Context(), path[2], input.DeviceLabel, time.Now())
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, request)
	case len(path) == 4 && path[0] == "v1" && path[1] == "pairings" && path[3] == "approve":
		if !methodIs(w, r, http.MethodPost) {
			return
		}
		ownerID, ok := h.requireOwner(w, r, true)
		if !ok {
			return
		}
		var input struct {
			RequestID      string `json:"request_id"`
			ComparisonCode string `json:"comparison_code"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		if err := store.ApprovePairing(r.Context(), ownerID, path[2], input.RequestID, input.ComparisonCode, time.Now()); err != nil {
			writeSyntheticError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case len(path) == 4 && path[0] == "v1" && path[1] == "pairings" && path[3] == "exchange":
		if !methodIs(w, r, http.MethodPost) {
			return
		}
		var input struct {
			RequestID  string `json:"request_id"`
			ClaimToken string `json:"claim_token"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		device, err := store.ExchangePairing(r.Context(), path[2], input.RequestID, input.ClaimToken, time.Now())
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, device)
	case r.URL.Path == "/v1/devices":
		if !methodIs(w, r, http.MethodGet) {
			return
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return
		}
		devices, err := store.ListDevices(r.Context(), ownerID)
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
	case len(path) == 4 && path[0] == "v1" && path[1] == "devices" && path[3] == "revoke":
		if !methodIs(w, r, http.MethodPost) {
			return
		}
		ownerID, ok := h.requireOwner(w, r, true)
		if !ok {
			return
		}
		if !emptyBody(w, r) {
			return
		}
		if err := store.RevokeDevice(r.Context(), ownerID, path[2], time.Now()); err != nil {
			writeSyntheticError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case len(path) == 4 && path[0] == "v1" && path[1] == "devices" && path[3] == "synthetic-pings":
		if !methodIs(w, r, http.MethodPost) {
			return
		}
		ownerID, ok := h.requireOwner(w, r, true)
		if !ok {
			return
		}
		if !emptyBody(w, r) {
			return
		}
		ping, err := store.CreatePing(r.Context(), ownerID, path[2], time.Now())
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, ping)
	case r.URL.Path == "/v1/synthetic-events":
		if !methodIs(w, r, http.MethodGet) {
			return
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return
		}
		events, err := store.ListEvents(r.Context(), ownerID)
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": events})
	case len(path) == 3 && path[0] == "v1" && path[1] == "synthetic-pings":
		if !methodIs(w, r, http.MethodGet) {
			return
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return
		}
		ping, err := store.GetPing(r.Context(), ownerID, path[2])
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, ping)
	case r.URL.Path == "/v1/agent/synthetic-sync":
		if !methodIs(w, r, http.MethodPost) {
			return
		}
		if _, err := r.Cookie(sessionCookieName); err == nil {
			writeBusinessProblem(w, http.StatusUnauthorized, "agent_credential_required")
			return
		}
		var input struct {
			ClientEventID string `json:"client_event_id"`
			AckPingID     string `json:"ack_ping_id"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") {
			writeBusinessProblem(w, http.StatusUnauthorized, "agent_credential_required")
			return
		}
		result, err := store.Sync(r.Context(), strings.TrimPrefix(authorization, "Bearer "),
			input.ClientEventID, input.AckPingID, time.Now())
		if err != nil {
			writeSyntheticError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	}
}

func (h *businessHandler) requireOwner(w http.ResponseWriter, r *http.Request, mutation bool) (string, bool) {
	if mutation && !h.validOrigin(r) {
		writeBusinessProblem(w, http.StatusForbidden, "origin_rejected")
		return "", false
	}
	ownerID, token, _, ok := h.currentSession(w, r)
	if !ok {
		return "", false
	}
	if mutation {
		expected, err := h.keys.CSRFToken(token)
		if err != nil {
			writeBusinessProblem(w, http.StatusServiceUnavailable, "key_unavailable")
			return "", false
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(expected)) != 1 {
			writeBusinessProblem(w, http.StatusForbidden, "csrf_rejected")
			return "", false
		}
	}
	return ownerID, true
}

func methodIs(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeBusinessProblem(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeBusinessProblem(w, http.StatusUnsupportedMediaType, "json_required")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

func emptyBody(w http.ResponseWriter, r *http.Request) bool {
	var one [1]byte
	count, err := http.MaxBytesReader(w, r.Body, 1).Read(one[:])
	if count == 0 && err == io.EOF {
		return true
	}
	writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func writeSyntheticError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, synthetic.ErrNotFound):
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	case errors.Is(err, synthetic.ErrConflict):
		writeBusinessProblem(w, http.StatusConflict, "conflict")
	case errors.Is(err, synthetic.ErrExpired):
		writeBusinessProblem(w, http.StatusGone, "pairing_expired")
	case errors.Is(err, synthetic.ErrDenied):
		writeBusinessProblem(w, http.StatusUnauthorized, "credential_denied")
	case errors.Is(err, synthetic.ErrCode):
		writeBusinessProblem(w, http.StatusForbidden, "comparison_code_rejected")
	default:
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
	}
}
