package config

import (
	"net"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		bad  bool
	}{
		{name: "defaults"},
		{name: "container", env: map[string]string{"RELAY_HTTP_ADDR": "0.0.0.0:8080"}},
		{name: "ipv6", env: map[string]string{"RELAY_HTTP_ADDR": "[::1]:8080"}},
		{name: "empty host", env: map[string]string{"RELAY_HTTP_ADDR": ":8080"}, bad: true},
		{name: "invalid port", env: map[string]string{"RELAY_HTTP_ADDR": "127.0.0.1:0"}, bad: true},
		{name: "empty database", env: map[string]string{"RELAY_DATABASE_PATH": " "}, bad: true},
		{name: "invalid log level", env: map[string]string{"RELAY_LOG_LEVEL": "trace"}, bad: true},
		// The mail allowlist must fail closed: malformed entries are
		// configuration errors, never a silently widened policy.
		{name: "allowlist entry without port", env: map[string]string{"RELAY_MAIL_PRIVATE_ALLOWLIST": "127.0.0.1"}, bad: true},
		{name: "allowlist empty entry", env: map[string]string{"RELAY_MAIL_PRIVATE_ALLOWLIST": "127.0.0.1:143,"}, bad: true},
		{name: "allowlist port zero", env: map[string]string{"RELAY_MAIL_PRIVATE_ALLOWLIST": "127.0.0.1:0"}, bad: true},
		{name: "allowlist port range", env: map[string]string{"RELAY_MAIL_PRIVATE_ALLOWLIST": "127.0.0.1:65536"}, bad: true},
		{name: "allowlist bare ipv6", env: map[string]string{"RELAY_MAIL_PRIVATE_ALLOWLIST": "::1:143"}, bad: true},
		{name: "allowlist host with spaces", env: map[string]string{"RELAY_MAIL_PRIVATE_ALLOWLIST": "127.0.0.1 :143"}, bad: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := load(func(key string) (string, bool) {
				value, exists := test.env[key]
				return value, exists
			})
			if (err != nil) != test.bad {
				t.Fatalf("unexpected validation result: %v", err)
			}
			if !test.bad && cfg.DatabasePath != "data/relay.db" {
				t.Fatalf("unexpected database default: %q", cfg.DatabasePath)
			}
		})
	}
}

func TestLoadMailPrivateAllowlist(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		set  bool
		want []MailEndpoint
	}{
		{name: "unset", raw: "", set: false},
		{name: "empty", raw: "", set: true},
		{name: "whitespace only", raw: "  \t ", set: true},
		{
			name: "two loopback endpoints",
			raw:  "127.0.0.1:143,127.0.0.1:587", set: true,
			want: []MailEndpoint{
				{Host: "127.0.0.1", IP: net.ParseIP("127.0.0.1"), Port: 143},
				{Host: "127.0.0.1", IP: net.ParseIP("127.0.0.1"), Port: 587},
			},
		},
		{
			name: "ipv6 literal and hostname", raw: "[::1]:993,mail.relay.test:143", set: true,
			want: []MailEndpoint{
				{Host: "::1", IP: net.ParseIP("::1"), Port: 993},
				{Host: "mail.relay.test", Port: 143},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := load(func(key string) (string, bool) {
				if key != "RELAY_MAIL_PRIVATE_ALLOWLIST" {
					return "", false
				}
				return test.raw, test.set
			})
			if err != nil {
				t.Fatalf("unexpected load error: %v", err)
			}
			if len(cfg.MailPrivateAllowlist) != len(test.want) {
				t.Fatalf("got %d entries, want %d: %+v", len(cfg.MailPrivateAllowlist), len(test.want), cfg.MailPrivateAllowlist)
			}
			for i, want := range test.want {
				got := cfg.MailPrivateAllowlist[i]
				if got.Host != want.Host || got.Port != want.Port || !got.IP.Equal(want.IP) {
					t.Fatalf("entry %d: got %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

func TestMailPrivateAllow(t *testing.T) {
	entries, err := parseMailAllowlist("127.0.0.1:143,127.0.0.1:587,[::1]:993,mail.relay.test:143")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		host string
		ip   string
		port int
		want bool
	}{
		{name: "exact listed endpoint", host: "127.0.0.1", ip: "127.0.0.1", port: 143, want: true},
		{name: "listed smtp submission endpoint", host: "127.0.0.1", ip: "127.0.0.1", port: 587, want: true},
		{name: "listed ipv6 endpoint", host: "::1", ip: "::1", port: 993, want: true},
		{name: "listed hostname exact", host: "mail.relay.test", ip: "127.0.0.1", port: 143, want: true},
		// An IP-literal entry also admits a configured host that resolves to
		// the same literal (for example "localhost" -> 127.0.0.1).
		{name: "ip literal matches resolved ip", host: "localhost", ip: "127.0.0.1", port: 143, want: true},
		{name: "ip literal matches mapped form", host: "other.relay.test", ip: "::ffff:127.0.0.1", port: 143, want: true},
		{name: "unlisted port", host: "127.0.0.1", ip: "127.0.0.1", port: 993, want: false},
		{name: "unlisted private host", host: "192.168.1.10", ip: "192.168.1.10", port: 143, want: false},
		{name: "hostname never matches ip entry", host: "mail.relay.test", ip: "127.0.0.1", port: 993, want: false},
		{name: "ip literal entry does not admit other ip", host: "localhost", ip: "127.8.8.8", port: 143, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := MailPrivateAllow(entries, test.host, net.ParseIP(test.ip), test.port)
			if got != test.want {
				t.Fatalf("MailPrivateAllow(%s, %s, %d) = %v, want %v", test.host, test.ip, test.port, got, test.want)
			}
		})
	}
	// No configuration at all is the production default: everything denied.
	if MailPrivateAllow(nil, "127.0.0.1", net.ParseIP("127.0.0.1"), 143) {
		t.Fatal("empty allowlist must deny every private endpoint")
	}
}

func TestLoadOptionalPaths(t *testing.T) {
	cfg, err := load(func(key string) (string, bool) {
		switch key {
		case "RELAY_MAIL_ROOT_CA_FILE":
			return " certs/dev-mail-ca.pem ", true
		case "RELAY_WEB_DIR":
			return "bin/web-dist", true
		}
		return "", false
	})
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if cfg.MailRootCAFile != "certs/dev-mail-ca.pem" {
		t.Fatalf("unexpected root CA file: %q", cfg.MailRootCAFile)
	}
	if cfg.WebDir != "bin/web-dist" {
		t.Fatalf("unexpected web dir: %q", cfg.WebDir)
	}
	if len(cfg.MailPrivateAllowlist) != 0 {
		t.Fatalf("unexpected allowlist entries: %+v", cfg.MailPrivateAllowlist)
	}
}
