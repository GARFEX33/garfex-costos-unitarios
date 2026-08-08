package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	valid := map[string]string{
		"GARFEX_DB_HOST": "localhost", "GARFEX_DB_PORT": "5432", "GARFEX_DB_NAME": "garfex", "GARFEX_DB_USER": "garfex_app", "GARFEX_DB_PASSWORD": "a-secret-value", "GARFEX_DB_SSLMODE": "disable",
	}
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		wantCode   int
		wantOut    string
		wantErr    string
		forbidText string
	}{
		{name: "no arguments shows usage", wantCode: 0, wantOut: "garfex", wantErr: "Usage:"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantOut: version},
		{name: "config check valid", args: []string{"config", "check"}, env: valid, wantCode: 0, wantOut: "configuration is valid", forbidText: "a-secret-value"},
		{name: "config check invalid", args: []string{"config", "check"}, wantCode: 1, wantErr: "GARFEX_DB_HOST"},
		{name: "unknown command", args: []string{"serve"}, wantCode: 2, wantErr: "unknown command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			gotCode := run(tt.args, mapLook(tt.env), &out, &errw)
			if gotCode != tt.wantCode {
				t.Errorf("run() code = %d, want %d", gotCode, tt.wantCode)
			}
			if !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want %q", out.String(), tt.wantOut)
			}
			if !strings.Contains(errw.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want %q", errw.String(), tt.wantErr)
			}
			if tt.forbidText != "" && strings.Contains(out.String()+errw.String(), tt.forbidText) {
				t.Errorf("output exposes secret: %q", out.String()+errw.String())
			}
		})
	}
}

func mapLook(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) { value, ok := values[name]; return value, ok }
}
