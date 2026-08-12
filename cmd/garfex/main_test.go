package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/tui"
	"github.com/charmbracelet/x/ansi"
)

type fakeProgram struct{ err error }

func (p fakeProgram) Run() (tea.Model, error) { return nil, p.err }

// fakeMaterialRepository is a stub domain.MaterialRepository: it is never
// invoked along the composition path exercised by TestRun, it only needs to
// satisfy the interface so a successful repositoryBuilder can be stubbed.
type fakeMaterialRepository struct{}

func (fakeMaterialRepository) Create(context.Context, domain.Material) error { return nil }
func (fakeMaterialRepository) Get(context.Context, string, string) (domain.Material, error) {
	return domain.Material{}, nil
}
func (fakeMaterialRepository) Search(context.Context, domain.SearchCriteria) ([]domain.Material, error) {
	return nil, nil
}

func TestRun(t *testing.T) {
	valid := map[string]string{
		"GARFEX_DB_HOST": "localhost", "GARFEX_DB_PORT": "5432", "GARFEX_DB_NAME": "garfex", "GARFEX_DB_USER": "garfex_app", "GARFEX_DB_PASSWORD": "a-secret-value", "GARFEX_DB_SSLMODE": "disable",
	}
	tests := []struct {
		name        string
		args        []string
		env         map[string]string
		wantCode    int
		wantOut     string
		wantErr     string
		exactOut    string
		exactErr    string
		forbidText  string
		launcher    programLauncher
		repoBuilder repositoryBuilder
	}{
		{name: "no arguments launch TUI", args: nil, env: valid, wantCode: 0, launcher: func(model tea.Model) program {
			m, ok := model.(tui.Model)
			if !ok {
				t.Fatal("launcher model must be the GARFEX TUI")
			}
			// Resize generously so the greeting is not scrolled/cut off by
			// the workspace viewport's default (small) height.
			resized, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 60})
			if plain := ansi.Strip(resized.View().Content); !strings.Contains(plain, "catálogo real") {
				t.Fatalf("launcher model view = %q, want it to contain the MaterialsWorkspaceAdapter greeting proving NewWithAgent wiring", plain)
			}
			return fakeProgram{}
		}},
		{name: "TUI launcher failure", args: nil, env: valid, wantCode: 1, wantErr: "TUI launcher failed: terminal unavailable", launcher: func(tea.Model) program { return fakeProgram{err: errors.New("terminal unavailable")} }},
		{name: "missing configuration does not launch TUI", args: nil, wantCode: 1, wantErr: "configuration is invalid", launcher: func(tea.Model) program {
			t.Fatal("launcher must not be invoked when configuration is invalid")
			return fakeProgram{}
		}},
		{name: "database unavailable does not launch TUI", args: nil, env: valid, wantCode: 1, wantErr: "database unavailable: connection refused",
			repoBuilder: func(context.Context, string) (domain.MaterialRepository, error) {
				return nil, errors.New("connection refused")
			},
			launcher: func(tea.Model) program {
				t.Fatal("launcher must not be invoked when the database is unavailable")
				return fakeProgram{}
			}},
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
			repoBuilder := tt.repoBuilder
			if repoBuilder == nil {
				repoBuilder = func(context.Context, string) (domain.MaterialRepository, error) {
					return fakeMaterialRepository{}, nil
				}
			}
			gotCode := run(tt.args, mapLook(tt.env), &out, &errw, launcher, repoBuilder)
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
