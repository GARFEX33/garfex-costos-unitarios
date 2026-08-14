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

// fakeResourceRepository is a stub domain.ResourceRepository: it is never
// invoked along the composition path exercised by TestRun, it only needs to
// satisfy the interface so a successful repositoryBuilder can be stubbed.
type fakeResourceRepository struct{}

func (fakeResourceRepository) Create(context.Context, domain.Resource) error { return nil }
func (fakeResourceRepository) Get(context.Context, string, string) (domain.Resource, error) {
	return domain.Resource{}, nil
}
func (fakeResourceRepository) Search(context.Context, domain.SearchCriteria) ([]domain.Resource, error) {
	return nil, nil
}
func (fakeResourceRepository) Update(context.Context, domain.Resource) error { return nil }
func (fakeResourceRepository) SetActive(context.Context, int64, bool) error  { return nil }

// brokenCatalog is a deliberately structurally-invalid domain.ResourceCatalog
// (a family referencing a class code the catalog never defines) — used to
// drive run()'s catalog.Validate() fail-fast path without touching the real
// seeded domain.SeedResourceCatalog().
func brokenCatalog() domain.ResourceCatalog {
	return domain.ResourceCatalog{
		Families: []domain.ResourceFamily{{ClassCode: "GHOST", Code: "X", Name: "X"}},
	}
}

func TestRun(t *testing.T) {
	valid := map[string]string{
		"GARFEX_DB_HOST": "localhost", "GARFEX_DB_PORT": "5432", "GARFEX_DB_NAME": "garfex", "GARFEX_DB_USER": "garfex_app", "GARFEX_DB_PASSWORD": "a-secret-value", "GARFEX_DB_SSLMODE": "disable",
	}
	tests := []struct {
		name           string
		args           []string
		env            map[string]string
		wantCode       int
		wantOut        string
		wantErr        string
		exactOut       string
		exactErr       string
		forbidText     string
		launcher       programLauncher
		repoBuilder    repositoryBuilder
		catalogBuilder catalogBuilder
	}{
		{name: "no arguments launch TUI", args: nil, env: valid, wantCode: 0, launcher: func(model tea.Model) program {
			m, ok := model.(tui.Model)
			if !ok {
				t.Fatal("launcher model must be the GARFEX TUI")
			}
			// Resize generously so the greeting is not scrolled/cut off by
			// the workspace viewport's default (small) height.
			resized, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 60})
			// The Assistant chat is the default active workspace at launch
			// (GARFEX / ASSISTANT stays a general shell and no longer
			// redirects to Materials on startup or on free text), so the
			// initial view shows AssistantShellAgent honest placeholder,
			// not the Materials greeting.
			if plain := ansi.Strip(resized.View().Content); !strings.Contains(plain, "Escribí / para elegir una capacidad") {
				t.Fatalf("launcher model view = %q, want it to contain the AssistantShellAgent placeholder proving NewWithAgents wiring", plain)
			}
			return fakeProgram{}
		}},
		{name: "TUI launcher failure", args: nil, env: valid, wantCode: 1, wantErr: "TUI launcher failed: terminal unavailable", launcher: func(tea.Model) program { return fakeProgram{err: errors.New("terminal unavailable")} }},
		{name: "missing configuration does not launch TUI", args: nil, wantCode: 1, wantErr: "configuration is invalid", launcher: func(tea.Model) program {
			t.Fatal("launcher must not be invoked when configuration is invalid")
			return fakeProgram{}
		}},
		{name: "database unavailable does not launch TUI", args: nil, env: valid, wantCode: 1, wantErr: "database unavailable: connection refused",
			repoBuilder: func(context.Context, string) (domain.ResourceRepository, error) {
				return nil, errors.New("connection refused")
			},
			launcher: func(tea.Model) program {
				t.Fatal("launcher must not be invoked when the database is unavailable")
				return fakeProgram{}
			}},
		// TestRun/invalid_resource_catalog_does_not_launch_TUI is the 9.1 RED
		// case: run() must fail fast (non-zero exit, no launch) when
		// catalog.Validate() reports a structural defect, using the same
		// injectable-dependency seam every other composition-failure case
		// above already uses — no real DB is touched, brokenCatalog() is a
		// pure in-memory fixture.
		{name: "invalid resource catalog does not launch TUI", args: nil, env: valid, wantCode: 1, wantErr: "catálogo de recursos inválido",
			catalogBuilder: brokenCatalog,
			launcher: func(tea.Model) program {
				t.Fatal("launcher must not be invoked when the resource catalog is invalid")
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
				repoBuilder = func(context.Context, string) (domain.ResourceRepository, error) {
					return fakeResourceRepository{}, nil
				}
			}
			catalogBuilder := tt.catalogBuilder
			if catalogBuilder == nil {
				catalogBuilder = domain.SeedResourceCatalog
			}
			gotCode := run(tt.args, mapLook(tt.env), &out, &errw, launcher, repoBuilder, catalogBuilder)
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
