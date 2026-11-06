// Package webapp serves the production web application build as static
// files from a directory on relay's own origin (RELAY_WEB_DIR). It is the
// same-origin topology of the local serving mode: relay runs on a loopback
// listener and hosts the built SPA next to the API. The security headers
// follow the poccollector web precedent; caching distinguishes the
// fingerprinted assets (immutable) from the HTML entry point (always
// revalidated). Directory contents are untrusted input served as-is; path
// handling never escapes the configured root.
package webapp

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// contentSecurityPolicy matches the poccollector web precedent. The SPA is a
// hash-routed Vue build: scripts, styles, images and connections all come
// from this origin (or inline data images), nothing else is needed.
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self'; img-src data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

const assetsPrefix = "/assets/"

// contentTypeByExtension pins the served media types instead of consulting
// the platform MIME registry, which is not deterministic on Windows.
var contentTypeByExtension = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".json":  "application/json",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".ico":   "image/x-icon",
	".txt":   "text/plain; charset=utf-8",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".map":   "application/json",
}

// New returns a handler serving the web application build from dir. The
// build must contain an index.html entry point; anything else is a
// configuration error so relay never starts half-deployed. The directory
// root is opened per request: path handling never escapes the configured
// root, and no long-lived handle keeps the directory locked (Windows allows
// a redeploy while relay is running).
func New(dir string) (http.Handler, error) {
	info, err := os.Stat(path.Join(dir, "index.html"))
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("web directory %q does not contain an index.html build", dir)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root, err := os.OpenRoot(dir)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer root.Close()
		serve(w, r, root.FS())
	}), nil
}

func serve(w http.ResponseWriter, r *http.Request, files fs.FS) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	requested := r.URL.Path
	if requested == "/index.html" {
		// The entry point is the SPA fallback target; serve it on "/" only.
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(requested, assetsPrefix) {
		serveFile(w, r, files, strings.TrimPrefix(requested, "/"), "public, max-age=31536000, immutable")
		return
	}
	if path.Ext(requested) != "" {
		// Extension-bearing paths outside /assets/ are never part of the
		// SPA; do not leak arbitrary build files (or mask real 404s).
		http.NotFound(w, r)
		return
	}
	// Extensionless paths (and "/") fall back to the entry point so hash
	// routing and deep links keep working.
	serveFile(w, r, files, "index.html", "no-cache")
}

func serveFile(w http.ResponseWriter, r *http.Request, files fs.FS, name, cacheControl string) {
	w.Header().Set("Cache-Control", cacheControl)
	if !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(files, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := contentTypeByExtension[path.Ext(name)]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	modTime := time.Time{}
	if info, err := fs.Stat(files, name); err == nil {
		modTime = info.ModTime()
	}
	// ServeContent also answers HEAD and range requests for free.
	http.ServeContent(w, r, path.Base(name), modTime, bytes.NewReader(data))
}
