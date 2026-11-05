package poccollector

import (
	"bytes"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strings"
	"time"
)

// Require loopback peers and literal-loopback Host to block proxy exposure and DNS rebinding.
// Browser writes remain forbidden; reads use this origin without CORS.
func localRequest(r *http.Request) bool {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(peer).IsLoopback() {
		return false
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil || !net.ParseIP(host).IsLoopback() || port == "" {
		return false
	}
	site := r.Header.Get("Sec-Fetch-Site")
	if site != "" && site != "same-origin" && site != "none" {
		return false
	}
	origin := r.Header.Get("Origin")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return origin == ""
	}
	return origin == "" || origin == "http://"+r.Host
}

func serveWeb(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		collectorError(w, 405, "method_not_allowed")
		return
	}
	name, contentType := "index.html", "text/html; charset=utf-8"
	if r.URL.Path != "/" {
		name = strings.TrimPrefix(r.URL.Path, "/")
		if !fs.ValidPath(name) || path.Dir(name) != "assets" || strings.HasPrefix(path.Base(name), ".") {
			collectorError(w, 404, "not_found")
			return
		}
		switch path.Ext(name) {
		case ".js":
			contentType = "text/javascript; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		default:
			collectorError(w, 404, "not_found")
			return
		}
	}
	data, err := fs.ReadFile(assets, name)
	if err != nil {
		collectorError(w, 404, "not_found")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
