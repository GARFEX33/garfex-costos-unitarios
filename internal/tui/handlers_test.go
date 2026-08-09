package tui

import (
	"strings"
	"testing"
)

func TestHandlers(t *testing.T) {
	valid := func(name string) (string, bool) {
		values := map[string]string{"GARFEX_DB_HOST": "db", "GARFEX_DB_PORT": "5432", "GARFEX_DB_NAME": "name", "GARFEX_DB_USER": "user", "GARFEX_DB_PASSWORD": "secret", "GARFEX_DB_SSLMODE": "require"}
		v, ok := values[name]
		return v, ok
	}
	for _, tt := range []struct {
		name    string
		handler Handler
		want    string
		wantErr bool
	}{
		{"version", Version("v1.2.3"), "v1.2.3", false},
		{"config success", Config(valid), "configuration is valid\npassword: ***REDACTED***", false},
		{"config failure", Config(func(string) (string, bool) { return "secret", false }), "configuration is invalid: GARFEX_DB_HOST", true},
		{"status", Status(), "Version: available\nConfig check: available\nTUI menu: available", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.handler()().(resultMsg)
			if (msg.err != nil) != tt.wantErr {
				t.Fatalf("error = %v", msg.err)
			}
			got := msg.text
			if msg.err != nil {
				got = msg.err.Error()
			}
			if got != tt.want {
				t.Fatalf("output = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, "secret") || strings.Contains(got, "DB, migration") {
				t.Fatalf("unsafe output: %q", got)
			}
		})
	}
}
