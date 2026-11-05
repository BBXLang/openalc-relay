// Package config reads the small set of settings supported by the bootstrap server.
package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr      string
	DatabasePath  string
	MasterKeyFile string
	PublicOrigin  string
	LogLevel      slog.Level
}

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, fallback string) string {
		if value, exists := lookup(key); exists {
			return value
		}
		return fallback
	}
	cfg := Config{
		HTTPAddr:      get("RELAY_HTTP_ADDR", "127.0.0.1:8080"),
		DatabasePath:  get("RELAY_DATABASE_PATH", "data/relay.db"),
		MasterKeyFile: get("RELAY_MASTER_KEY_FILE", ""),
		PublicOrigin:  get("RELAY_PUBLIC_ORIGIN", ""),
	}
	host, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil || net.ParseIP(host) == nil {
		return Config{}, fmt.Errorf("RELAY_HTTP_ADDR must contain a literal IP address and port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("RELAY_HTTP_ADDR port must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.DatabasePath) == "" {
		return Config{}, fmt.Errorf("RELAY_DATABASE_PATH must not be empty")
	}
	switch get("RELAY_LOG_LEVEL", "info") {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "info":
		cfg.LogLevel = slog.LevelInfo
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		return Config{}, fmt.Errorf("RELAY_LOG_LEVEL must be debug, info, warn, or error")
	}
	return cfg, nil
}
