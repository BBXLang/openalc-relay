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
	// MailPrivateAllowlist holds the parsed RELAY_MAIL_PRIVATE_ALLOWLIST
	// entries. A nil/empty list keeps the production default: no private or
	// loopback mail endpoint is ever admitted (deny by default).
	MailPrivateAllowlist []MailEndpoint
	// MailRootCAFile is the optional RELAY_MAIL_ROOT_CA_FILE PEM bundle used
	// to verify mail-server certificates signed by a private (test
	// environment) CA. Empty keeps the system root store.
	MailRootCAFile string
	// WebDir is the optional RELAY_WEB_DIR static build of the production
	// web application, served by relay on its own (loopback-only) origin.
	WebDir string
}

// MailEndpoint is one exact allowlist entry of RELAY_MAIL_PRIVATE_ALLOWLIST.
// Host is the configured endpoint host; IP is non-nil exactly when Host is an
// IP literal. A hostname entry only ever matches that exact host string; an
// IP-literal entry additionally matches any configured host whose resolved IP
// equals the entry's literal (for example "localhost" resolving to
// 127.0.0.1). Everything else stays denied.
type MailEndpoint struct {
	Host string
	IP   net.IP
	Port int
}

// MailPrivateAllow reports whether an exact mail target (configured host,
// resolved IP, port) is named by one of the allowlist entries. It is the
// matcher behind the mailadapter EndpointPolicy PrivateAllow hook: the port
// must be identical and the host must either equal the entry's configured
// host or, for an IP-literal entry, resolve to that same literal. Service
// kind is deliberately not part of the match: the entries are exact network
// endpoints the administrator chose to admit. With no entries this always
// returns false, which is the unchanged production default.
func MailPrivateAllow(entries []MailEndpoint, host string, ip net.IP, port int) bool {
	for _, entry := range entries {
		if entry.Port != port {
			continue
		}
		if entry.Host == host {
			return true
		}
		if entry.IP != nil && ip != nil && entry.IP.Equal(ip) {
			return true
		}
	}
	return false
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
	// RELAY_MAIL_PRIVATE_ALLOWLIST widens the mail endpoint policy only for
	// the exact host:port endpoints listed here (RFC-0003 §5: 私网邮件主机仅
	// 通过服务器管理员显式允许列表接入). Unset, empty or whitespace-only means
	// the production deny-by-default behavior: no private or loopback mail
	// target is ever admitted.
	allowlist, err := parseMailAllowlist(get("RELAY_MAIL_PRIVATE_ALLOWLIST", ""))
	if err != nil {
		return Config{}, err
	}
	cfg.MailPrivateAllowlist = allowlist
	// RELAY_MAIL_ROOT_CA_FILE optionally swaps the trusted root store for the
	// mail connections (verification itself is never skipped); empty keeps
	// the system roots.
	cfg.MailRootCAFile = strings.TrimSpace(get("RELAY_MAIL_ROOT_CA_FILE", ""))
	// RELAY_WEB_DIR optionally serves the production web application build on
	// the same origin; empty leaves the pure API surface unchanged.
	cfg.WebDir = strings.TrimSpace(get("RELAY_WEB_DIR", ""))
	return cfg, nil
}

// parseMailAllowlist parses RELAY_MAIL_PRIVATE_ALLOWLIST: a comma-separated
// list of exact "host:port" endpoints (for example "127.0.0.1:143,127.0.0.1:587").
// Each entry must carry a literal host (or RFC 1123 hostname) and a port in
// 1..65535; anything else fails configuration so a typo can never silently
// widen the policy. An unset, empty or whitespace-only value yields nil
// entries: nothing private is allowed.
func parseMailAllowlist(raw string) ([]MailEndpoint, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var endpoints []MailEndpoint
	for _, field := range strings.Split(raw, ",") {
		field = strings.TrimSpace(field)
		host, portText, err := net.SplitHostPort(field)
		if err != nil || host == "" || strings.ContainsAny(host, " \t\"'") {
			return nil, fmt.Errorf("RELAY_MAIL_PRIVATE_ALLOWLIST entry %q must be a host:port endpoint", field)
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("RELAY_MAIL_PRIVATE_ALLOWLIST entry %q has no valid port", field)
		}
		entry := MailEndpoint{Host: host, Port: port}
		if ip := net.ParseIP(host); ip != nil {
			entry.IP = ip
		}
		endpoints = append(endpoints, entry)
	}
	return endpoints, nil
}
