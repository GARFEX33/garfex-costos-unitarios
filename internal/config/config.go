// Package config loads and validates GARFEX runtime configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Secret prevents credentials from being accidentally printed or serialized.
type Secret string

func (Secret) String() string   { return "***REDACTED***" }
func (Secret) GoString() string { return "***REDACTED***" }

// MarshalText prevents text formats from exposing a secret.
func (Secret) MarshalText() ([]byte, error) { return []byte("***REDACTED***"), nil }

// MarshalJSON prevents JSON output from exposing a secret.
func (Secret) MarshalJSON() ([]byte, error) { return json.Marshal("***REDACTED***") }

// Reveal returns the underlying value. It is the only intentional escape hatch.
func (s Secret) Reveal() string { return string(s) }

// Config contains the validated runtime connection settings.
type Config struct {
	DBHost     string
	DBPort     int
	DBName     string
	DBUser     string
	DBPassword Secret
	DBSSLMode  string
	LogLevel   string
}

// DSN builds a postgres connection string from the validated fields. It
// necessarily contains the real password since it is a real connection
// string; callers must not log or print it.
func (c Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(c.DBUser), url.QueryEscape(c.DBPassword.Reveal()), c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}

// ValidationError identifies an invalid environment variable without its value.
type ValidationError struct {
	Var    string
	Reason string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Var, e.Reason)
}

// Load reads only GARFEX runtime variables from look and validates them.
func Load(look func(string) (string, bool)) (Config, error) {
	var cfg Config
	var validationErrors []error

	required := func(name string) string {
		value, ok := look(name)
		if !ok || strings.TrimSpace(value) == "" {
			validationErrors = append(validationErrors, ValidationError{Var: name, Reason: "is required"})
			return ""
		}
		return value
	}

	cfg.DBHost = required("GARFEX_DB_HOST")
	port := required("GARFEX_DB_PORT")
	cfg.DBName = required("GARFEX_DB_NAME")
	cfg.DBUser = required("GARFEX_DB_USER")
	cfg.DBPassword = Secret(required("GARFEX_DB_PASSWORD"))
	cfg.DBSSLMode = required("GARFEX_DB_SSLMODE")

	if port != "" {
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed < 1 || parsed > 65535 {
			validationErrors = append(validationErrors, ValidationError{Var: "GARFEX_DB_PORT", Reason: "must be an integer between 1 and 65535"})
		} else {
			cfg.DBPort = parsed
		}
	}
	if cfg.DBSSLMode != "" && !oneOf(cfg.DBSSLMode, "disable", "require", "verify-ca", "verify-full") {
		validationErrors = append(validationErrors, ValidationError{Var: "GARFEX_DB_SSLMODE", Reason: "must be disable, require, verify-ca, or verify-full"})
	}

	cfg.LogLevel = "info"
	if value, ok := look("GARFEX_LOG_LEVEL"); ok && strings.TrimSpace(value) != "" {
		cfg.LogLevel = value
	}
	if !oneOf(cfg.LogLevel, "debug", "info", "warn", "error") {
		validationErrors = append(validationErrors, ValidationError{Var: "GARFEX_LOG_LEVEL", Reason: "must be debug, info, warn, or error"})
	}

	if len(validationErrors) > 0 {
		return Config{}, errors.Join(validationErrors...)
	}
	return cfg, nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
