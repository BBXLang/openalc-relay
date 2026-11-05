// poc-collector is a loopback-only test harness for synthetic Android events.
package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/OpenALC/relay/server/internal/poccollector"
)

func main() { os.Exit(run()) }

func run() int {
	address := flag.String("addr", "127.0.0.1:8787", "Loopback listener only")
	path := flag.String("database", "data/poc-collector.db", "Separate synthetic database path")
	dropACK := flag.Bool("drop-first-ack", false, "Commit then drop the first response for fault injection")
	webDir := flag.String("web-dir", "", "Optional built synthetic Web assets directory")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	host, _, err := net.SplitHostPort(*address)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		logger.Error("collector_requires_literal_loopback_address")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	store, err := poccollector.Open(startup, *path)
	cancel()
	if err != nil {
		logger.Error("collector_storage_initialization_failed")
		return 1
	}
	defer store.Close()
	var assets fs.FS
	if *webDir != "" {
		assets = os.DirFS(filepath.Clean(*webDir))
		if _, err := fs.ReadFile(assets, "index.html"); err != nil {
			logger.Error("collector_web_assets_unavailable")
			return 1
		}
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		logger.Error("collector_listener_failed")
		return 1
	}
	server := &http.Server{
		Handler: poccollector.NewWebHandler(store, *dropACK, assets), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
		MaxHeaderBytes: 16 << 10,
	}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	logger.Info("synthetic_collector_started", "address", listener.Addr().String(), "drop_first_ack", *dropACK)
	select {
	case err := <-result:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("collector_serve_failed")
			return 1
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			server.Close()
			logger.Error("collector_shutdown_failed")
			return 1
		}
		<-result
	}
	return 0
}
