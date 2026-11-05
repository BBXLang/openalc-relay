package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/OpenALC/relay/server/internal/mailstore"
	"github.com/OpenALC/relay/server/internal/mailsync"
)

// This file exposes the batch-3 manual mail sync slice
// (FORMAL-UI-MAIL-PLAN): an owner-session user can trigger one bounded INBOX
// sync round for one of their accounts, or probe the stored credentials with
// a pure connection test. Both are POST mutations of the mail surface and
// therefore require Origin, CSRF token and a valid session exactly like the
// other state-changing routes. Nothing here logs: credentials, message
// content and remote protocol details stay out of every response, and the
// sync layer keeps them out of its errors.

// serveMailSync handles the two sync actions under /v1/mail-accounts and
// reports whether the request belonged to this surface. It runs before
// serveMail in the dispatch order, which owns the remaining mail-accounts
// routes.
func (h *businessHandler) serveMailSync(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(path) != 4 || path[0] != "v1" || path[1] != "mail-accounts" ||
		(path[3] != "sync" && path[3] != "connection-test") {
		return false
	}
	if !methodIs(w, r, http.MethodPost) {
		return true
	}
	ownerID, ok := h.requireOwner(w, r, true)
	if !ok {
		return true
	}
	if !emptyBody(w, r) {
		return true
	}
	syncer := &mailsync.Syncer{
		Store:   &mailstore.Store{DB: h.db, Keys: h.keys},
		Keys:    h.keys,
		RootCAs: h.mailRootCAs,
	}
	switch path[3] {
	case "sync":
		result, err := syncer.SyncInbox(r.Context(), ownerID, path[2], h.mailPolicy)
		if err != nil {
			writeMailSyncError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, result)
	case "connection-test":
		if err := syncer.TestConnection(r.Context(), ownerID, path[2], h.mailPolicy); err != nil {
			writeMailSyncError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "imap_ready": true})
	}
	return true
}

// writeMailSyncError maps sync failures to stable problem codes. The
// categories follow the plan's failure semantics: endpoint-policy denials
// are the server refusing the target, upstream authentication rejections and
// connectivity failures are gateway-class answers, storage and keying
// failures never look like a successful sync, and no error carries internal
// detail (wrapped causes, credentials or message content stay server-side).
func writeMailSyncError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mailstore.ErrNotFound):
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	case errors.Is(err, mailstore.ErrDenied):
		writeBusinessProblem(w, http.StatusForbidden, "account_revoked")
	case errors.Is(err, mailstore.ErrCiphertext):
		writeBusinessProblem(w, http.StatusInternalServerError, "decryption_failed")
	case errors.Is(err, mailsync.ErrPolicy):
		writeBusinessProblem(w, http.StatusForbidden, "endpoint_policy_denied")
	case errors.Is(err, mailsync.ErrAuth):
		writeBusinessProblem(w, http.StatusBadGateway, "upstream_auth_failed")
	case errors.Is(err, mailsync.ErrConnect):
		writeBusinessProblem(w, http.StatusBadGateway, "upstream_connect_failed")
	case errors.Is(err, mailsync.ErrFetch):
		writeBusinessProblem(w, http.StatusBadGateway, "upstream_fetch_failed")
	default:
		// Storage, keying and unknown failures: an honest 503, never a
		// partial success.
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
	}
}
