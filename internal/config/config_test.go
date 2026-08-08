package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	valid := map[string]string{
		"GARFEX_DB_HOST":     "localhost",
		"GARFEX_DB_PORT":     "5432",
		"GARFEX_DB_NAME":     "garfex",
		"GARFEX_DB_USER":     "garfex_app",
		"GARFEX_DB_PASSWORD": "a-secret-value",
		"GARFEX_DB_SSLMODE":  "disable",
	}

	tests := []struct {
		name    string
		env     map[string]string
		wantErr []string
		check   func(t *testing.T, got Config)
	}{
		{
			name: "valid environment uses default log level",
			env:  valid,
			check: func(t *testing.T, got Config) {
				t.Helper()
				if got.DBHost != "localhost" || got.DBPort != 5432 || got.DBName != "garfex" || got.DBUser != "garfex_app" || got.DBPassword.Reveal() != "a-secret-value" || got.DBSSLMode != "disable" || got.LogLevel != "info" {
					t.Fatalf("Load() = %#v, want validated values", got)
				}
			},
		},
		{
			name:    "missing required values are aggregated",
			env:     map[string]string{},
			wantErr: []string{"GARFEX_DB_HOST", "GARFEX_DB_PORT", "GARFEX_DB_NAME", "GARFEX_DB_USER", "GARFEX_DB_PASSWORD", "GARFEX_DB_SSLMODE"},
		},
		{
			name:    "invalid port",
			env:     with(valid, "GARFEX_DB_PORT", "70000"),
			wantErr: []string{"GARFEX_DB_PORT"},
		},
		{
			name:    "invalid ssl mode",
			env:     with(valid, "GARFEX_DB_SSLMODE", "unsafe"),
			wantErr: []string{"GARFEX_DB_SSLMODE"},
		},
		{
			name:    "invalid log level",
			env:     with(valid, "GARFEX_LOG_LEVEL", "verbose"),
			wantErr: []string{"GARFEX_LOG_LEVEL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(mapLook(tt.env))
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("Load() error = %v", err)
				}
				tt.check(t, got)
				return
			}
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
			var validationErr ValidationError
			if !errors.As(err, &validationErr) {
				t.Errorf("Load() error = %v, want a ValidationError", err)
			}
			for _, variable := range tt.wantErr {
				if !strings.Contains(err.Error(), variable) {
					t.Errorf("Load() error = %v, want validation error for %s", err, variable)
				}
			}
		})
	}
}

func TestSecretIsRedacted(t *testing.T) {
	secret := Secret("a-secret-value")
	for _, rendered := range []string{fmt.Sprintf("%s", secret), fmt.Sprintf("%v", secret), fmt.Sprintf("%+v", secret), fmt.Sprintf("%#v", secret)} {
		if strings.Contains(rendered, secret.Reveal()) || rendered != "***REDACTED***" {
			t.Fatalf("secret rendered as %q", rendered)
		}
	}
	encoded, err := json.Marshal(struct {
		Password Secret `json:"password"`
	}{Password: secret})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(encoded), secret.Reveal()) || !strings.Contains(string(encoded), "***REDACTED***") {
		t.Fatalf("json.Marshal() = %s, secret was not redacted", encoded)
	}
}

func TestEnvExampleDocumentsRuntimeVariables(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile(.env.example) error = %v", err)
	}
	documented := make(map[string]bool)
	for _, line := range strings.Split(string(contents), "\n") {
		name, _, ok := strings.Cut(line, "=")
		if ok && strings.HasPrefix(name, "GARFEX_") {
			documented[name] = true
		}
	}
	for _, variable := range []string{"GARFEX_DB_HOST", "GARFEX_DB_PORT", "GARFEX_DB_NAME", "GARFEX_DB_USER", "GARFEX_DB_PASSWORD", "GARFEX_DB_SSLMODE", "GARFEX_LOG_LEVEL"} {
		if !documented[variable] {
			t.Errorf(".env.example does not document %s", variable)
		}
	}
}

func mapLook(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) { value, ok := values[name]; return value, ok }
}

func with(values map[string]string, key, value string) map[string]string {
	copy := make(map[string]string, len(values)+1)
	for k, v := range values {
		copy[k] = v
	}
	copy[key] = value
	return copy
}
