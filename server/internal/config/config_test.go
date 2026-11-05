package config

import "testing"

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
