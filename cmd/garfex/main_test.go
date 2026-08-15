package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/postgres"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/tui"
	"github.com/charmbracelet/x/ansi"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeProgram struct{ err error }

func (p fakeProgram) Run() (tea.Model, error) { return nil, p.err }

// fakeResourceRepository is a stub domain.ResourceRepository: it is never
// invoked along the composition path exercised by TestRun, it only needs to
// satisfy the interface so a successful infraBuilder can be stubbed.
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

// fakeInfraBuilder is a successful infraBuilder that never touches a real
// Postgres instance: nil pool (ignored by the stub catalogLoader used
// alongside it) plus fakeResourceRepository.
func fakeInfraBuilder(context.Context, string) (*pgxpool.Pool, domain.ResourceRepository, error) {
	return nil, fakeResourceRepository{}, nil
}

// fakeCatalogLoader is a successful catalogLoader backed by
// domain.SeedResourceCatalog(), ignoring the (possibly nil) pool — the test
// seam design §2 calls out surviving the boot rewiring unchanged.
func fakeCatalogLoader(context.Context, *pgxpool.Pool) (domain.ResourceCatalog, error) {
	return domain.SeedResourceCatalog(), nil
}

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
		name          string
		args          []string
		env           map[string]string
		wantCode      int
		wantOut       string
		wantErr       string
		exactOut      string
		exactErr      string
		forbidText    string
		launcher      programLauncher
		infraBuilder  infraBuilder
		catalogLoader catalogLoader
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
		// New boot order (design §2): config -> buildInfra (DB connect) ->
		// loadCatalog -> catalog.Validate(). "database unavailable" moves to
		// the buildInfra step and its diagnostic is now Spanish, matching
		// the other two catalog-boot diagnostics below.
		{name: "database unavailable does not launch TUI", args: nil, env: valid, wantCode: 1, wantErr: "base de datos no disponible: connection refused",
			infraBuilder: func(context.Context, string) (*pgxpool.Pool, domain.ResourceRepository, error) {
				return nil, nil, errors.New("connection refused")
			},
			launcher: func(tea.Model) program {
				t.Fatal("launcher must not be invoked when the database is unavailable")
				return fakeProgram{}
			}},
		// 2.4 RED case 1/3: an empty catalog (migrations applied, never
		// seeded) must produce a distinct diagnostic from a generic load
		// failure, driven through postgres.ErrCatalogEmpty via a stub
		// catalogLoader — no real empty database is touched.
		{name: "empty resource catalog does not launch TUI", args: nil, env: valid, wantCode: 1, wantErr: "catálogo de recursos vacío",
			catalogLoader: func(context.Context, *pgxpool.Pool) (domain.ResourceCatalog, error) {
				return domain.ResourceCatalog{}, postgres.ErrCatalogEmpty
			},
			launcher: func(tea.Model) program {
				t.Fatal("launcher must not be invoked when the resource catalog is empty")
				return fakeProgram{}
			}},
		// 2.4 RED case 2/3: any OTHER catalogLoader failure (not
		// ErrCatalogEmpty) gets the generic load-failure diagnostic
		// (design §2's second loadCatalog branch).
		{name: "resource catalog load failure does not launch TUI", args: nil, env: valid, wantCode: 1, wantErr: "no se pudo cargar el catálogo de recursos: connection reset",
			catalogLoader: func(context.Context, *pgxpool.Pool) (domain.ResourceCatalog, error) {
				return domain.ResourceCatalog{}, errors.New("connection reset")
			},
			launcher: func(tea.Model) program {
				t.Fatal("launcher must not be invoked when the resource catalog fails to load")
				return fakeProgram{}
			}},
		// 2.4 RED case 3/3 (existed pre-rewiring, kept green through the
		// new seam): a structurally invalid catalog still fails fast, using
		// the same injectable-dependency seam every other
		// composition-failure case above uses — no real DB is touched,
		// brokenCatalog() is a pure in-memory fixture.
		{name: "invalid resource catalog does not launch TUI", args: nil, env: valid, wantCode: 1, wantErr: "catálogo de recursos inválido",
			catalogLoader: func(context.Context, *pgxpool.Pool) (domain.ResourceCatalog, error) {
				return brokenCatalog(), nil
			},
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
			infra := tt.infraBuilder
			if infra == nil {
				infra = fakeInfraBuilder
			}
			loadCatalog := tt.catalogLoader
			if loadCatalog == nil {
				loadCatalog = fakeCatalogLoader
			}
			gotCode := run(tt.args, mapLook(tt.env), &out, &errw, launcher, infra, loadCatalog)
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
