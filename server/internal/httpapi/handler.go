// Package httpapi exposes only the endpoints actually implemented by the server.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type health struct {
	Status string `json:"status"`
}

type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
}

func NewHandler(checkDatabase func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			writeProblem(w, r, http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeProblem(w, r, http.StatusMethodNotAllowed)
			return
		}
		status, body := http.StatusOK, health{Status: "ok"}
		if r.URL.Path == "/readyz" {
			ctx, cancel := context.WithTimeout(r.Context(), time.Second)
			defer cancel()
			if err := checkDatabase(ctx); err != nil {
				status, body = http.StatusServiceUnavailable, health{Status: "unavailable"}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if r.Method != http.MethodHead {
			json.NewEncoder(w).Encode(body)
		}
	})
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		json.NewEncoder(w).Encode(problem{Type: "about:blank", Title: http.StatusText(status), Status: status})
	}
}
