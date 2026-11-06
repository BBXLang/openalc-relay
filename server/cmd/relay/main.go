package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/OpenALC/relay/server/internal/config"
	"github.com/OpenALC/relay/server/internal/httpapi"
	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/mailadapter"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
	"github.com/OpenALC/relay/server/internal/webapp"
)

func main() {
	if code := run(); code != 0 {
		os.Exit(code)
	}
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
		logger.Error("configuration_invalid", "error", err.Error())
		return 1
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	db, err := sqlite.Open(startup, cfg.DatabasePath)
	cancel()
	if err != nil {
		// Database errors can contain paths or SQL; do not print raw storage errors.
		logger.Error("database_initialization_failed", "action", "check database permissions and migration compatibility")
		return 1
	}
	defer db.Close()
	initialized, err := identity.IsInitialized(ctx, db)
	if err != nil {
		logger.Error("owner_initialization_check_failed")
		return 1
	}
	var handler http.Handler = httpapi.NewHandler(db.PingContext)
	if initialized {
		if cfg.MasterKeyFile == "" || cfg.PublicOrigin == "" {
			logger.Error("business_configuration_missing")
			return 1
		}
		host, _, _ := net.SplitHostPort(cfg.HTTPAddr)
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			logger.Error("business_listener_must_be_loopback")
			return 1
		}
		keys, err := secrets.LoadFile(cfg.MasterKeyFile)
		if err != nil {
			logger.Error("business_key_file_invalid")
			return 1
		}
		// Mail connection policy: the strict production default is an empty
		// EndpointPolicy (no private or loopback target). Only an explicitly
		// configured allowlist (RELAY_MAIL_PRIVATE_ALLOWLIST) installs the
		// PrivateAllow hook, and only a configured private CA file
		// (RELAY_MAIL_ROOT_CA_FILE) swaps the trusted root store; both stay
		// nil otherwise, byte-identical to the previous behavior.
		var mailPolicy mailadapter.EndpointPolicy
		if len(cfg.MailPrivateAllowlist) > 0 {
			allowlist := cfg.MailPrivateAllowlist
			mailPolicy.PrivateAllow = func(_ mailadapter.ServiceKind, host string, resolved net.IP, port int) bool {
				return config.MailPrivateAllow(allowlist, host, resolved, port)
			}
		}
		var mailRootCAs *x509.CertPool
		if cfg.MailRootCAFile != "" {
			mailRootCAs, err = loadMailRootCAs(cfg.MailRootCAFile)
			if err != nil {
				logger.Error("mail_root_ca_file_invalid")
				return 1
			}
		}
		handler, err = httpapi.NewBusinessHandlerWithMailSync(db.PingContext, db, keys, cfg.PublicOrigin, mailPolicy, mailRootCAs)
		if err != nil {
			logger.Error("business_origin_invalid")
			return 1
		}
	} else if cfg.MasterKeyFile != "" || cfg.PublicOrigin != "" {
		logger.Error("owner_not_initialized")
		return 1
	}
	if cfg.WebDir != "" {
		handler, err = composeRoot(handler, cfg.WebDir)
		if err != nil {
			logger.Error("web_dir_invalid")
			return 1
		}
	}
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		logger.Error("http_listener_failed")
		return 1
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	phase := "bootstrap"
	if initialized {
		phase = "owner-session"
	}
	logger.Info("server_started", "address", listener.Addr().String(), "phase", phase)
	select {
	case err := <-result:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http_serve_failed")
			return 1
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			server.Close()
			logger.Error("http_shutdown_failed")
			return 1
		}
		if err := <-result; !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http_serve_failed")
			return 1
		}
	}
	logger.Info("server_stopped")
	return 0
}

// composeRoot layers the static web application in front of the business
// handler when RELAY_WEB_DIR is set. Priority stays with the API: /healthz,
// /readyz and every /v1 path are served by the business handler exactly as
// before; everything else falls through to the web application, which serves
// /assets/* with long-lived caching and falls back to index.html for
// extensionless GET paths (the SPA uses hash routing). Without RELAY_WEB_DIR
// the business handler is returned unchanged, byte-identical behavior.
func composeRoot(business http.Handler, webDir string) (http.Handler, error) {
	if webDir == "" {
		return business, nil
	}
	web, err := webapp.New(webDir)
	if err != nil {
		return nil, err
	}
	return rootHandler{business: business, web: web}, nil
}

type rootHandler struct {
	business http.Handler
	web      http.Handler
}

func (h rootHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "/healthz" || p == "/readyz" || p == "/v1" || strings.HasPrefix(p, "/v1/") {
		h.business.ServeHTTP(w, r)
		return
	}
	h.web.ServeHTTP(w, r)
}

// loadMailRootCAs reads a PEM certificate bundle for verifying mail servers
// with a private (test environment) CA. Verification itself is never skipped:
// the bundle only swaps the trust store.
func loadMailRootCAs(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	found := false
	for rest := data; len(rest) > 0; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		pool.AddCert(cert)
		found = true
	}
	if !found {
		return nil, errors.New("no PEM certificate found")
	}
	return pool, nil
}
