package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/OpenALC/relay/server/internal/mailstore"
)

// serveMail handles the implemented mail account management routes and
// reports whether the request belonged to this surface. Mail data exchange
// (IMAP/SMTP, inbox sync, send) is not implemented yet and stays unreachable.
func (h *businessHandler) serveMail(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(path) < 2 || path[0] != "v1" || path[1] != "mail-accounts" {
		return false
	}
	store := mailstore.Store{DB: h.db, Keys: h.keys}
	switch {
	case r.URL.Path == "/v1/mail-accounts" && r.Method == http.MethodPost:
		ownerID, ok := h.requireOwner(w, r, true)
		if !ok {
			return true
		}
		var input struct {
			Address  string `json:"address"`
			IMAPHost string `json:"imap_host"`
			IMAPPort int    `json:"imap_port"`
			SMTPHost string `json:"smtp_host"`
			SMTPPort int    `json:"smtp_port"`
			TLSMode  string `json:"tls_mode"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !decodeJSON(w, r, &input) {
			return true
		}
		account, err := store.CreateAccount(r.Context(), ownerID, mailstore.CreateAccountInput{
			Address: input.Address, IMAPHost: input.IMAPHost, IMAPPort: input.IMAPPort,
			SMTPHost: input.SMTPHost, SMTPPort: input.SMTPPort, TLSMode: input.TLSMode,
			Username: input.Username, Password: input.Password,
		}, time.Now())
		if err != nil {
			writeMailError(w, err)
			return true
		}
		writeJSON(w, http.StatusCreated, account)
	case r.URL.Path == "/v1/mail-accounts" && r.Method == http.MethodGet:
		ownerID, ok := h.requireOwner(w, r, false)
		if !ok {
			return true
		}
		accounts, err := store.ListAccounts(r.Context(), ownerID)
		if err != nil {
			writeMailError(w, err)
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
	case len(path) == 3 && r.Method == http.MethodDelete:
		ownerID, ok := h.requireOwner(w, r, true)
		if !ok {
			return true
		}
		if !emptyBody(w, r) {
			return true
		}
		if err := store.DeleteAccount(r.Context(), ownerID, path[2], time.Now()); err != nil {
			writeMailError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writeBusinessProblem(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
	return true
}

func writeMailError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mailstore.ErrNotFound):
		writeBusinessProblem(w, http.StatusNotFound, "not_found")
	case errors.Is(err, mailstore.ErrConflict):
		writeBusinessProblem(w, http.StatusConflict, "conflict")
	case errors.Is(err, mailstore.ErrDenied):
		writeBusinessProblem(w, http.StatusForbidden, "account_revoked")
	case errors.Is(err, mailstore.ErrInvalid):
		writeBusinessProblem(w, http.StatusBadRequest, "invalid_request")
	default:
		// Storage, keying and ciphertext failures must never look successful.
		writeBusinessProblem(w, http.StatusServiceUnavailable, "storage_unavailable")
	}
}
