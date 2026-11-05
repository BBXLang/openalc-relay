package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OpenALC/relay/server/internal/config"
	"github.com/OpenALC/relay/server/internal/httpapi"
	"github.com/OpenALC/relay/server/internal/identity"
	"github.com/OpenALC/relay/server/internal/secrets"
	"github.com/OpenALC/relay/server/internal/storage/sqlite"
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
		handler, err = httpapi.NewBusinessHandler(db.PingContext, db, keys, cfg.PublicOrigin)
		if err != nil {
			logger.Error("business_origin_invalid")
			return 1
		}
	} else if cfg.MasterKeyFile != "" || cfg.PublicOrigin != "" {
		logger.Error("owner_not_initialized")
		return 1
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
