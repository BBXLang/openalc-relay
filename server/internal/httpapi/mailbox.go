package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OpenALC/relay/server/internal/mailstore"
	"github.com/OpenALC/relay/server/internal/smsstore"
)

// This file exposes the batch-2 encrypted storage layers as read-only HTTP
// slices (FORMAL-UI-MAIL-PLAN batch 3, RFC-0003 reading subset). Every route
// requires a valid owner session cookie; pure GET requests only check the
// session and never need Origin or CSRF tokens because they change nothing.
// No write route exists on these paths: any other method answers 405. All
// queries enforce the owner scope in the store, so records of other owners are
// indistinguishable from missing records and always answer 404 (or an empty
// page for optional filters). Mail and SMS bodies stay sealed at rest and are
// decrypted per request; nothing here opens network connections, sends
// messages or starts synchronization.

// serveMailbox handles the read-only mail message routes and reports whether
// the request belonged to this surface.
func (h *businessHandler) serveMailbox(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(path) < 3 || path[0] != "v1" || path[1] != "mail" {
		return false
	}
	store := mailstore.Store{DB: h.db, Keys: h.keys}
	switch {
	case len(path) == 5 && path[2] == "accounts" && path[4] == "messages":
		if !methodIs(w, r, http.MethodGet) {
			return true
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return true
		}
		params, ok := parseListQuery(w, r, "cursor", "limit")
		if !ok {
			return true
		}
		limit, ok := parseLimit(w, params)
		if !ok {
			return true
		}
		// The account is the addressed resource: an unknown account or one
		// owned by somebody else must answer 404 instead of an empty page.
		if _, err := store.GetAccount(r.Context(), ownerID, path[3]); err != nil {
			writeMailReadError(w, err)
			return true
		}
		page, err := store.ListMessages(r.Context(), ownerID, mailstore.ListOptions{
			AccountID: path[3], Cursor: params.Get("cursor"), Limit: limit,
		})
		if err != nil {
			writeMailReadError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, page)
	case len(path) == 4 && path[2] == "messages":
		if !methodIs(w, r, http.MethodGet) {
			return true
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return true
		}
		params, ok := parseListQuery(w, r, "format")
		if !ok {
			return true
		}
		format := params.Get("format")
		if format != "" && format != "html" {
			writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
			return true
		}
		message, err := store.GetMessage(r.Context(), ownerID, path[3])
		if err != nil {
			writeMailReadError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, mailDetail(message, format))
	case len(path) == 3 && path[2] == "search":
		if !methodIs(w, r, http.MethodGet) {
			return true
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return true
		}
		params, ok := parseListQuery(w, r, "q", "account_id", "cursor")
		if !ok {
			return true
		}
		// RFC-0003 §6: the search term is decrypted-and-matched inside the
		// authorized scope only and must never reach any log. This handler
		// performs no logging at all, and the query is never echoed into
		// error responses.
		query := params.Get("q")
		if strings.TrimSpace(query) == "" {
			writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
			return true
		}
		page, err := store.SearchMessages(r.Context(), ownerID, mailstore.SearchOptions{
			AccountID: params.Get("account_id"), Query: query, Cursor: params.Get("cursor"),
		})
		if err != nil {
			writeMailReadError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, page)
	default:
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	}
	return true
}

// serveSMS handles the read-only SMS routes and reports whether the request
// belonged to this surface.
func (h *businessHandler) serveSMS(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(path) < 3 || path[0] != "v1" || path[1] != "sms" {
		return false
	}
	store := smsstore.Store{DB: h.db, Keys: h.keys}
	switch {
	case len(path) == 3 && path[2] == "messages":
		if !methodIs(w, r, http.MethodGet) {
			return true
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return true
		}
		params, ok := parseListQuery(w, r, "device_id", "cursor", "limit")
		if !ok {
			return true
		}
		limit, ok := parseLimit(w, params)
		if !ok {
			return true
		}
		page, err := store.ListMessages(r.Context(), ownerID, smsstore.ListOptions{
			DeviceID: params.Get("device_id"), Cursor: params.Get("cursor"), Limit: limit,
		})
		if err != nil {
			writeSMSReadError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, page)
	case len(path) == 4 && path[2] == "messages":
		if !methodIs(w, r, http.MethodGet) {
			return true
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return true
		}
		if _, ok := parseListQuery(w, r); !ok {
			return true
		}
		message, err := store.GetMessage(r.Context(), ownerID, path[3])
		if err != nil {
			writeSMSReadError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, message)
	case len(path) == 3 && path[2] == "search":
		if !methodIs(w, r, http.MethodGet) {
			return true
		}
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return true
		}
		params, ok := parseListQuery(w, r, "q", "device_id")
		if !ok {
			return true
		}
		// Same red-line as mail search: the query stays in memory only.
		query := params.Get("q")
		if strings.TrimSpace(query) == "" {
			writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
			return true
		}
		matches, err := store.Search(r.Context(), ownerID, smsstore.SearchOptions{
			DeviceID: params.Get("device_id"), Query: query,
		})
		if err != nil {
			writeSMSReadError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": matches})
	default:
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	}
	return true
}

// mailMessageDetail is the controlled detail representation of one stored mail
// message. The stored body never leaves the server unmediated: text bodies are
// returned as body_text; a stored HTML body is only returned on an explicit
// format=html request, exactly as stored, and always accompanied by the
// html_notice warning. NOTE: the dedicated HTML sanitizer is not implemented
// yet. A later batch must plug the sanitizeHtml pass in here under the same
// origin/content policy as the web client; until then this raw passthrough is
// the documented interim behavior and the notice field keeps it honest.
type mailMessageDetail struct {
	MessageID       string    `json:"message_id"`
	AccountID       string    `json:"account_id"`
	Folder          string    `json:"folder"`
	RemoteMessageID string    `json:"remote_message_id"`
	ReceivedAt      time.Time `json:"received_at"`
	SizeBytes       int64     `json:"size_bytes"`
	Unread          bool      `json:"unread"`
	HasAttachments  bool      `json:"has_attachments"`
	AttachmentCount int       `json:"attachment_count"`
	Subject         string    `json:"subject"`
	From            string    `json:"from"`
	To              []string  `json:"to"`
	Cc              []string  `json:"cc,omitempty"`
	BodyFormat      string    `json:"body_format"`
	BodyText        string    `json:"body_text,omitempty"`
	BodyHTML        string    `json:"body_html,omitempty"`
	HTMLNotice      string    `json:"html_notice,omitempty"`
}

const (
	htmlNoticeStored = "stored html body is not returned by default; request format=html to retrieve the stored representation"
	htmlNoticeRaw    = "html body returned exactly as stored; no additional sanitizer is applied at this layer yet"
)

func mailDetail(message mailstore.Message, format string) mailMessageDetail {
	detail := mailMessageDetail{
		MessageID: message.ID, AccountID: message.AccountID, Folder: message.Folder,
		RemoteMessageID: message.RemoteMessageID, ReceivedAt: message.ReceivedAt,
		SizeBytes: message.SizeBytes, Unread: message.Unread,
		HasAttachments: message.HasAttachments, AttachmentCount: message.AttachmentCount,
		Subject: message.Subject, From: message.Headers.From,
		To: message.Headers.To, Cc: message.Headers.Cc, BodyFormat: message.BodyFormat,
	}
	if detail.To == nil {
		detail.To = []string{}
	}
	// Text is preferred whenever the stored body is text; HTML is served only
	// on the explicit opt-in request described above.
	if message.BodyFormat == mailstore.BodySanitizedHTML {
		if format == "html" {
			detail.BodyHTML = message.Body
			detail.HTMLNotice = htmlNoticeRaw
		} else {
			detail.HTMLNotice = htmlNoticeStored
		}
	} else {
		detail.BodyText = message.Body
	}
	return detail
}

// parseListQuery validates that the request query carries only the named
// parameters. The business router rejects every query string by default; the
// read endpoints opt in with this explicit allow-list so unknown parameters
// can never silently alter results.
func parseListQuery(w http.ResponseWriter, r *http.Request, allowed ...string) (url.Values, bool) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
		return nil, false
	}
	for key := range values {
		if !slices.Contains(allowed, key) {
			writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
			return nil, false
		}
	}
	return values, true
}

// parseLimit bounds the page size to 1..100; an unset or empty limit falls
// back to the storage default page (20).
func parseLimit(w http.ResponseWriter, values url.Values) (int, bool) {
	raw := values.Get("limit")
	if raw == "" {
		return 0, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 100 {
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
		return 0, false
	}
	return limit, true
}

func writeMailReadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mailstore.ErrNotFound):
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	case errors.Is(err, mailstore.ErrInvalid):
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, mailstore.ErrCiphertext):
		// Tampered or wrongly keyed ciphertext fails the whole request; a
		// partial page or a zero-value record is never returned as success.
		writeBusinessProblem(w, http.StatusInternalServerError, "decryption_failed")
	default:
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
	}
}

func writeSMSReadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, smsstore.ErrNotFound):
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	case errors.Is(err, smsstore.ErrInvalid):
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, smsstore.ErrCiphertext):
		writeBusinessProblem(w, http.StatusInternalServerError, "decryption_failed")
	default:
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
	}
}

// readQueryAllowed lists the read endpoints that accept query parameters
// (pagination, scope filters, search and the HTML detail opt-in). Every other
// business route keeps the strict no-query rule.
func readQueryAllowed(path string) bool {
	return path == "/v1/mail/search" || path == "/v1/sms/search" ||
		path == "/v1/sms/messages" ||
		strings.HasPrefix(path, "/v1/mail/messages/") ||
		strings.HasPrefix(path, "/v1/mail/accounts/") ||
		strings.HasPrefix(path, "/v1/sms/messages/")
}
