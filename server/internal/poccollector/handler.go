package poccollector

import (
	"encoding/json"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
)

// NewHandler rejects non-loopback peers and unknown fields. dropFirstACK is explicit fault injection.
func NewHandler(store *Store, dropFirstACK bool) http.Handler {
	return NewWebHandler(store, dropFirstACK, nil)
}

// NewWebHandler optionally serves a built synthetic verification page on the same origin.
func NewWebHandler(store *Store, dropFirstACK bool, assets fs.FS) http.Handler {
	var dropped atomic.Bool
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !localRequest(r) {
			collectorError(w, 403, "loopback_only")
			return
		}
		if assets != nil && (r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/assets/")) {
			serveWeb(w, r, assets)
			return
		}
		if r.URL.Path == "/poc/v1/stats" && r.Method == http.MethodGet {
			count, err := store.Count(r.Context())
			if err != nil {
				collectorError(w, 503, "storage_unavailable")
				return
			}
			json.NewEncoder(w).Encode(map[string]int{"event_count": count})
			return
		}
		if r.URL.Path != "/poc/v1/events" {
			collectorError(w, 404, "not_found")
			return
		}
		if r.Method == http.MethodGet {
			query, err := url.ParseQuery(r.URL.RawQuery)
			limit := 25
			valid := err == nil
			for key, values := range query {
				if len(values) != 1 || (key != "installation_id" && key != "after" && key != "limit") {
					valid = false
				}
			}
			if values, exists := query["limit"]; exists && len(values) == 1 {
				limit, err = strconv.Atoi(values[0])
				valid = valid && err == nil
			}
			installationID, after := query.Get("installation_id"), query.Get("after")
			if !valid || !uuid.MatchString(installationID) || (query.Has("after") && !uuid.MatchString(after)) || limit < 1 || limit > 50 {
				collectorError(w, 400, "invalid_synthetic_query")
				return
			}
			page, err := store.List(r.Context(), installationID, after, limit)
			if err != nil {
				collectorError(w, 503, "storage_unavailable")
				return
			}
			json.NewEncoder(w).Encode(page)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, POST")
			collectorError(w, 405, "method_not_allowed")
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			collectorError(w, 415, "json_required")
			return
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		var batch Batch
		if err := decoder.Decode(&batch); err != nil || !validBatch(batch) {
			collectorError(w, 400, "invalid_synthetic_batch")
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			collectorError(w, 400, "invalid_synthetic_batch")
			return
		}
		receipt, err := store.Persist(r.Context(), batch)
		if err != nil {
			collectorError(w, 503, "storage_unavailable")
			return
		}
		if dropFirstACK && dropped.CompareAndSwap(false, true) {
			// Commit has completed. Closing the socket recreates an ACK-lost crash window.
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close()
					return
				}
			}
			collectorError(w, 503, "ack_injected_failure")
			return
		}
		json.NewEncoder(w).Encode(receipt)
	})
}

func collectorError(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"code": code})
}
