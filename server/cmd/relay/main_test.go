package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/OpenALC/relay/server/internal/config"
	"github.com/OpenALC/relay/server/internal/mailadapter"
)

func TestInitializedProcessRequiresKeysAndCanLogIn(t *testing.T) {
	dir := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	serverBin := filepath.Join(dir, "relay"+suffix)
	adminBin := filepath.Join(dir, "relay-admin"+suffix)
	build := func(path, output string) {
		t.Helper()
		cmd := exec.Command("go", "build", "-o", output, path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v: %s", path, err, out)
		}
	}
	build(".", serverBin)
	build("../relay-admin", adminBin)
	keyPath := filepath.Join(dir, "keys.json")
	dbPath := filepath.Join(dir, "relay.db")
	passwordPath := filepath.Join(dir, "password.txt")
	password := "SyntheticOnly-Owner-Password-2026"
	if err := os.WriteFile(passwordPath, []byte(password), 0o600); err != nil {
		t.Fatal(err)
	}
	runAdmin := func(args ...string) {
		t.Helper()
		cmd := exec.Command(adminBin, args...)
		if err := cmd.Run(); err != nil {
			t.Fatalf("admin command failed: %v", err)
		}
	}
	runAdmin("keygen", "-out", keyPath)
	runAdmin("bootstrap", "-database", dbPath, "-key-file", keyPath, "-password-file", passwordPath)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	origin := "https://" + address
	baseEnv := append(os.Environ(),
		"RELAY_DATABASE_PATH="+dbPath, "RELAY_HTTP_ADDR="+address,
		"RELAY_MASTER_KEY_FILE=", "RELAY_PUBLIC_ORIGIN=")
	missing := exec.Command(serverBin)
	missing.Env = baseEnv
	if err := missing.Run(); err == nil {
		t.Fatal("initialized server started without key and HTTPS origin")
	}
	server := exec.Command(serverBin)
	server.Env = append(baseEnv,
		"RELAY_MASTER_KEY_FILE="+keyPath, "RELAY_PUBLIC_ORIGIN="+origin)
	var logs bytes.Buffer
	server.Stdout = &logs
	server.Stderr = &logs
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		server.Process.Kill()
		server.Wait()
	}()
	client := &http.Client{Timeout: time.Second}
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		resp, err := client.Get("http://" + address + "/readyz")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("initialized server did not become ready: %s", logs.String())
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+address+"/v1/session",
		strings.NewReader(fmt.Sprintf(`{"password":%q}`, password)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner login status = %d", resp.StatusCode)
	}
	if len(resp.Cookies()) != 1 || resp.Cookies()[0].Name != "__Host-relay-session" ||
		!resp.Cookies()[0].Secure || !resp.Cookies()[0].HttpOnly {
		t.Fatal("owner login did not set a secure session cookie")
	}
}

// stubBusiness marks which handler answered and echoes its status; it stands
// in for the real API handler (whose /v1 and health routes are covered by the
// httpapi tests).
func stubBusiness(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handler", "business")
		w.WriteHeader(status)
		w.Write([]byte("business:" + r.URL.Path))
	})
}

func writeComposeWebDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>relay</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash("assets/app.9z.js")), []byte("let a=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func composeRequest(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1"+target, nil)
	r.URL.Path = target
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestComposeRootWithoutWebDirIsUnchanged(t *testing.T) {
	business := stubBusiness(http.StatusNotFound)
	handler, err := composeRoot(business, "")
	if err != nil {
		t.Fatal(err)
	}
	// Without RELAY_WEB_DIR every path, including "/" and static-looking
	// paths, reaches the business handler exactly as before.
	for _, target := range []string{"/", "/index.html", "/assets/app.9z.js", "/healthz", "/v1/mail-accounts"} {
		w := composeRequest(handler, http.MethodGet, target)
		if w.Header().Get("X-Handler") != "business" {
			t.Fatalf("target %s handled outside the business handler", target)
		}
		if w.Code != http.StatusNotFound {
			t.Fatalf("target %s: got %d, want %d", target, w.Code, http.StatusNotFound)
		}
	}
}

func TestComposeRootServesWebAppOnSameOrigin(t *testing.T) {
	handler, err := composeRoot(stubBusiness(http.StatusTeapot), writeComposeWebDir(t))
	if err != nil {
		t.Fatal(err)
	}
	// Business priority is preserved: health and every /v1 path.
	for _, target := range []string{"/healthz", "/readyz", "/v1/session", "/v1/mail-accounts/abc/connection-test"} {
		w := composeRequest(handler, http.MethodGet, target)
		if w.Code != http.StatusTeapot {
			t.Fatalf("target %s did not reach the business handler: %d", target, w.Code)
		}
	}
	// The web application owns everything else.
	root := composeRequest(handler, http.MethodGet, "/")
	if root.Code != http.StatusOK || root.Body.String() != "<html>relay</html>" {
		t.Fatalf("root: %d %q", root.Code, root.Body.String())
	}
	fallback := composeRequest(handler, http.MethodGet, "/mailbox/inbox")
	if fallback.Code != http.StatusOK || fallback.Body.String() != "<html>relay</html>" {
		t.Fatalf("SPA fallback: %d %q", fallback.Code, fallback.Body.String())
	}
	asset := composeRequest(handler, http.MethodGet, "/assets/app.9z.js")
	if asset.Code != http.StatusOK || asset.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset: %d %q", asset.Code, asset.Header().Get("Cache-Control"))
	}
	if w := composeRequest(handler, http.MethodGet, "/assets/missing.9z.js"); w.Code != http.StatusNotFound {
		t.Fatalf("missing asset: %d", w.Code)
	}
	if w := composeRequest(handler, http.MethodPost, "/"); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST to web surface: %d", w.Code)
	}
}

func TestComposeRootRejectsInvalidWebDir(t *testing.T) {
	if _, err := composeRoot(stubBusiness(http.StatusOK), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("empty web directory must be rejected")
	}
}

// TestMailPolicyWiringFromConfig proves the allowlist hook built from config
// entries admits exactly the listed endpoints through the EndpointPolicy
// validate/dial path: listed loopback endpoints pass, everything else private
// keeps the default denial, and the IP-literal clause admits a configured
// host resolving to the entry's literal (an IPv4-mapped form here, so no DNS
// is involved).
func TestMailPolicyWiringFromConfig(t *testing.T) {
	t.Setenv("RELAY_MAIL_PRIVATE_ALLOWLIST", "127.0.0.1:143,127.0.0.1:587")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	entries := cfg.MailPrivateAllowlist
	if len(entries) != 2 {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	var policy mailadapter.EndpointPolicy
	if len(entries) > 0 {
		policy.PrivateAllow = func(_ mailadapter.ServiceKind, host string, ip net.IP, port int) bool {
			return config.MailPrivateAllow(entries, host, ip, port)
		}
	}
	ctx := context.Background()
	if err := policy.Validate(ctx, mailadapter.ServiceIMAP, "127.0.0.1", 143); err != nil {
		t.Fatalf("allowlisted IMAP endpoint: %v", err)
	}
	if err := policy.Validate(ctx, mailadapter.ServiceSMTP, "127.0.0.1", 587); err != nil {
		t.Fatalf("allowlisted SMTP endpoint: %v", err)
	}
	if err := policy.Validate(ctx, mailadapter.ServiceIMAP, "127.0.0.1", 993); !errors.Is(err, mailadapter.ErrPrivateTarget) {
		t.Fatalf("unlisted loopback endpoint must stay denied: %v", err)
	}
	if err := policy.Validate(ctx, mailadapter.ServiceSMTP, "192.168.1.10", 587); !errors.Is(err, mailadapter.ErrPrivateTarget) {
		t.Fatalf("unlisted private host must stay denied: %v", err)
	}
	// The IP-literal clause: a host whose resolved IP equals the entry's
	// literal is admitted (IPv4-mapped loopback, no resolver involved).
	if err := policy.Validate(ctx, mailadapter.ServiceIMAP, "::ffff:127.0.0.1", 143); err != nil {
		t.Fatalf("resolved-IP match: %v", err)
	}
	// Without the allowlist the hook is absent: same targets all denied.
	if err := (mailadapter.EndpointPolicy{}).Validate(ctx, mailadapter.ServiceIMAP, "127.0.0.1", 143); !errors.Is(err, mailadapter.ErrPrivateTarget) {
		t.Fatalf("default policy must deny loopback: %v", err)
	}
}
