package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/OpenALC/relay/server/internal/smsstore"
)

// This file implements the owner side of the RFC-0003 §3 single-send command
// surface: an owner session can enqueue exactly one explicitly confirmed SMS
// send command for one of their own devices, read its state back and page
// through the command list. Every create carries a mandatory per-command user
// confirmation, a per-owner idempotency key and two rate limits (the batch-2
// queued-command ceiling in the store plus a request window here), and the
// create is audited in the store transaction.
//
// The device claim and result-reporting halves of the protocol are still
// Proposed (RFC-0003 is not adopted), so commands created here honestly stay
// queued: no endpoint in this file moves a command out of queued and no
// automatic transition exists anywhere. Nothing here contacts a device, a
// mobile network or any sending API. Recipient and body are sealed at rest
// and never echoed: create, detail and list responses carry command state
// metadata only, and the same red-line holds for every error response.

const (
	// idempotency key bounds mirror the batch-2 sms_send_commands schema
	// CHECK (length BETWEEN 16 AND 128): printable ASCII only.
	minIdempotencyKeyBytes = 16
	maxIdempotencyKeyBytes = 128
	// sendCommandRateLimit bounds validated command-create requests per
	// owner per fixed minute window (RFC-0003 §3 I4: request-level limiting
	// on top of the store's queued-command ceiling). It sits above
	// MaxQueuedCommandsPerOwner so the queue limit stays observable.
	sendCommandRateLimit    = 60
	sendCommandWindowLength = time.Minute
)

// serveSMSCommands handles the owner send-command routes and reports whether
// the request belonged to this surface. It runs before serveSMS in the
// dispatch order, which owns the remaining /v1/sms read routes.
func (h *businessHandler) serveSMSCommands(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(path) < 3 || path[0] != "v1" || path[1] != "sms" || path[2] != "send-commands" {
		return false
	}
	switch {
	case len(path) == 3:
		switch r.Method {
		case http.MethodPost:
			h.createSendCommand(w, r)
		case http.MethodGet:
			h.listSendCommands(w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			writeBusinessProblem(w, http.StatusMethodNotAllowed, "method_not_allowed")
		}
	case len(path) == 4:
		if !methodIs(w, r, http.MethodGet) {
			return true
		}
		h.getSendCommand(w, r, path[3])
	default:
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	}
	return true
}

// createSendCommand enqueues one single-send command. An identical retry
// (same owner key, same device and content) returns the existing command with
// idempotent_replay=true; the same key with different content is a 409 that
// leaves the stored command untouched. The response never contains the
// recipient or the body.
func (h *businessHandler) createSendCommand(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.requireOwner(w, r, true)
	if !ok {
		return
	}
	// The list endpoint shares this path and needs query parameters; the
	// create endpoint does not, and silently ignoring them would be worse.
	if r.URL.RawQuery != "" {
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
		return
	}
	var input struct {
		DeviceID       string `json:"device_id"`
		Recipient      string `json:"recipient"`
		Body           string `json:"body"`
		IdempotencyKey string `json:"idempotency_key"`
		Confirm        bool   `json:"confirm"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	// AGENTS.md: remote sending requires an explicit user operation and no
	// unconfirmed batch sending exists. A missing or false confirm is
	// rejected before any state changes or rate budget is consumed.
	if !input.Confirm {
		writeBusinessProblem(w, http.StatusBadRequest, "missing_confirmation")
		return
	}
	if len(input.IdempotencyKey) < minIdempotencyKeyBytes ||
		len(input.IdempotencyKey) > maxIdempotencyKeyBytes || !printableASCII(input.IdempotencyKey) {
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if !h.allowSendCommand(time.Now(), ownerID) {
		w.Header().Set("Retry-After", "60")
		writeBusinessProblem(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	store := smsstore.Store{DB: h.db, Keys: h.keys}
	command, status, err := store.CreateCommand(r.Context(), ownerID, smsstore.CommandInput{
		DeviceID: input.DeviceID, IdempotencyKey: input.IdempotencyKey,
		Recipient: input.Recipient, Body: input.Body,
	}, time.Now())
	if err != nil {
		writeSendCommandError(w, err)
		return
	}
	replay := status == smsstore.StatusDuplicate
	code := http.StatusCreated
	if replay {
		code = http.StatusOK
	}
	writeJSON(w, code, smsCommandCreated{
		CommandID: command.ID, Status: command.Status, CreatedAt: command.CreatedAt,
		IdempotentReplay: replay,
	})
}

// getSendCommand returns the state view of one command inside the owner
// scope; commands of other owners are indistinguishable from missing ones.
func (h *businessHandler) getSendCommand(w http.ResponseWriter, r *http.Request, commandID string) {
	ownerID, ok := h.requireOwner(w, r, false)
	if !ok {
		return
	}
	if _, ok := parseListQuery(w, r); !ok {
		return
	}
	store := smsstore.Store{DB: h.db, Keys: h.keys}
	command, err := store.GetCommand(r.Context(), ownerID, commandID)
	if err != nil {
		writeSendCommandError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, smsCommandState{
		CommandID: command.ID, DeviceID: command.DeviceID, Status: command.Status,
		CreatedAt: command.CreatedAt, ResolvedAt: command.ResolvedAt,
	})
}

// listSendCommands pages through the owner's command states with the shared
// keyset cursor, optionally filtered by device and a whitelisted status.
func (h *businessHandler) listSendCommands(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.requireOwner(w, r, false)
	if !ok {
		return
	}
	params, ok := parseListQuery(w, r, "device_id", "status", "cursor", "limit")
	if !ok {
		return
	}
	limit, ok := parseLimit(w, params)
	if !ok {
		return
	}
	status := params.Get("status")
	if status != "" && !smsstore.ValidCommandStatus(status) {
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
		return
	}
	store := smsstore.Store{DB: h.db, Keys: h.keys}
	page, err := store.ListCommands(r.Context(), ownerID, smsstore.CommandListOptions{
		DeviceID: params.Get("device_id"), Status: status,
		Cursor: params.Get("cursor"), Limit: limit,
	})
	if err != nil {
		writeSendCommandError(w, err)
		return
	}
	commands := make([]smsCommandState, 0, len(page.Commands))
	for _, command := range page.Commands {
		commands = append(commands, smsCommandState{
			CommandID: command.ID, DeviceID: command.DeviceID, Status: command.Status,
			CreatedAt: command.CreatedAt, ResolvedAt: command.ResolvedAt,
		})
	}
	writeJSON(w, http.StatusOK, smsCommandPage{Commands: commands, NextCursor: page.NextCursor})
}

// smsCommandState is the controlled state view of one send command. It
// deliberately has no recipient, body or idempotency key field: command state
// responses never carry message content or retry keys.
type smsCommandState struct {
	CommandID  string     `json:"command_id"`
	DeviceID   string     `json:"device_id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// smsCommandCreated is the create response. idempotent_replay is only
// present (true) on the 200 replay of an identical earlier command.
type smsCommandCreated struct {
	CommandID        string    `json:"command_id"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	IdempotentReplay bool      `json:"idempotent_replay,omitempty"`
}

type smsCommandPage struct {
	Commands   []smsCommandState `json:"commands"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

// writeSendCommandError maps store failures to stable problem codes. Content
// is never part of any error: decryption failures fail closed, and unknown
// failures are an honest 503 instead of a partial success.
func writeSendCommandError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, smsstore.ErrNotFound):
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	case errors.Is(err, smsstore.ErrDenied):
		writeBusinessProblem(w, http.StatusForbidden, "device_revoked")
	case errors.Is(err, smsstore.ErrConflict):
		writeBusinessProblem(w, http.StatusConflict, "idempotency_conflict")
	case errors.Is(err, smsstore.ErrInvalid):
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, smsstore.ErrRateLimited):
		writeBusinessProblem(w, http.StatusTooManyRequests, "queue_limit")
	case errors.Is(err, smsstore.ErrCiphertext):
		writeBusinessProblem(w, http.StatusInternalServerError, "decryption_failed")
	default:
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
	}
}

// allowSendCommand applies the per-owner fixed-window request limit for
// command creation. Windows live in a map keyed by owner id: the process
// serves one bootstrapped owner, so growth is bounded, and stale windows are
// pruned if that ever changes.
func (h *businessHandler) allowSendCommand(now time.Time, ownerID string) bool {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	window, ok := h.sendWindows[ownerID]
	if !ok || now.Sub(window.start) >= sendCommandWindowLength {
		window = sendCommandWindowState{start: now}
	}
	window.count++
	h.sendWindows[ownerID] = window
	if len(h.sendWindows) > 4096 {
		for id, stale := range h.sendWindows {
			if now.Sub(stale.start) >= sendCommandWindowLength {
				delete(h.sendWindows, id)
			}
		}
	}
	return window.count <= sendCommandRateLimit
}

type sendCommandWindowState struct {
	start time.Time
	count int
}

// printableASCII reports whether every byte is a printable ASCII character.
func printableASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] > 0x7E {
			return false
		}
	}
	return true
}
