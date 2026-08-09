package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/tui"
)

type fakeProgram struct{ err error }

func (p fakeProgram) Run() (tea.Model, error) { return nil, p.err }

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
		exactOut   string
		exactErr   string
		forbidText string
		launcher   programLauncher
	}{
		{name: "no arguments launch TUI", wantCode: 0, launcher: func(model tea.Model) program {
			if _, ok := model.(tui.Model); !ok {
				t.Fatal("launcher model must be the GARFEX TUI")
			}
			return fakeProgram{}
		}},
		{name: "TUI launcher failure", wantCode: 1, wantErr: "TUI launcher failed: terminal unavailable", launcher: func(tea.Model) program { return fakeProgram{err: errors.New("terminal unavailable")} }},
		{name: "version", args: []string{"version"}, wantCode: 0, wantOut: version, exactOut: version + "\n"},
		{name: "config check valid", args: []string{"config", "check"}, env: valid, wantCode: 0, wantOut: "configuration is valid", exactOut: "configuration is valid: host=localhost port=5432 name=garfex user=garfex_app sslmode=disable log_level=info password=***REDACTED***\n", forbidText: "a-secret-value"},
		{name: "config check invalid", args: []string{"config", "check"}, wantCode: 1, wantErr: "GARFEX_DB_HOST"},
		{name: "unknown command", args: []string{"serve"}, wantCode: 2, wantErr: "unknown command", exactErr: "unknown command: serve\nUsage: garfex [version | config check]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			launcher := tt.launcher
			if launcher == nil {
				launcher = func(tea.Model) program { return fakeProgram{} }
			}
			gotCode := run(tt.args, mapLook(tt.env), &out, &errw, launcher)
			if gotCode != tt.wantCode {
				t.Errorf("run() code = %d, want %d", gotCode, tt.wantCode)
			}
			if !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want %q", out.String(), tt.wantOut)
			}
			if !strings.Contains(errw.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want %q", errw.String(), tt.wantErr)
			}
			if tt.exactOut != "" && out.String() != tt.exactOut {
				t.Errorf("stdout = %q, want %q", out.String(), tt.exactOut)
			}
			if tt.exactErr != "" && errw.String() != tt.exactErr {
				t.Errorf("stderr = %q, want %q", errw.String(), tt.exactErr)
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
