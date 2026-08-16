package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/app/catalogo"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/app/recursos"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/config"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/postgres"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/tui"
	"github.com/jackc/pgx/v5/pgxpool"
)

var version = "dev"

type program interface {
	Run() (tea.Model, error)
}

type programLauncher func(tea.Model) program

// infraBuilder connects to Postgres and returns both the shared pool (which
// catalogLoader hydrates the catalog from) and the Resources repository
// built over it. It is injected so run() is unit-testable without a real
// Postgres instance — the retyped successor to the recursos-maestro PR9
// repositoryBuilder seam: LoadResourceCatalog needs the same pool the
// repository uses, so both come from one connect (design §2).
type infraBuilder func(ctx context.Context, dsn string) (*pgxpool.Pool, domain.ResourceRepository, error)

// catalogLoader hydrates the domain.ResourceCatalog run() validates and
// wires through the rest of the composition. It is injected (defaulting to
// postgres.LoadResourceCatalog in main()) so a stub can drive the empty-
// and invalid-catalog fail-fast paths in a test without a real Postgres
// instance (design §2; supersedes the retired catalogBuilder Go-literal
// seam now that Postgres is canonical for catalog structure).
type catalogLoader func(ctx context.Context, pool *pgxpool.Pool) (domain.ResourceCatalog, error)

func main() {
	os.Exit(run(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, newProgram, newPostgresInfra, postgres.LoadResourceCatalog))
}

func newProgram(model tea.Model) program { return tea.NewProgram(model) }

func newPostgresInfra(ctx context.Context, dsn string) (*pgxpool.Pool, domain.ResourceRepository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, postgres.NewResourceRepository(pool), nil
}

func run(args []string, look func(string) (string, bool), out, errw io.Writer, launch programLauncher, buildInfra infraBuilder, loadCatalog catalogLoader) int {
	if len(args) == 0 {
		cfg, err := config.Load(look)
		if err != nil {
			fmt.Fprintf(errw, "configuration is invalid: %v\n", err)
			return 1
		}
		pool, repo, err := buildInfra(context.Background(), cfg.DSN())
		if err != nil {
			fmt.Fprintf(errw, "base de datos no disponible: %v\n", err)
			return 1
		}
		catalog, err := loadCatalog(context.Background(), pool)
		if err != nil {
			if errors.Is(err, postgres.ErrCatalogEmpty) {
				fmt.Fprintf(errw, "catálogo de recursos vacío: ejecuta las migraciones (scripts/db/migrate.sh up)\n")
			} else {
				fmt.Fprintf(errw, "no se pudo cargar el catálogo de recursos: %v\n", err)
			}
			return 1
		}
		// Fail fast: a structurally malformed catalog (e.g. a future data-only
		// class addition with a dangling reference) must never reach the TUI
		// silently misbehaving — it aborts the launch instead (design §8).
		if err := catalog.Validate(); err != nil {
			fmt.Fprintf(errw, "catálogo de recursos inválido: %v\n", err)
			return 1
		}
		authority := domain.NewCatalogAuthority(catalog)
		service := recursos.NewServiceWithCatalogAuthority(repo, authority)
		agentFor := func(snapshot domain.ResourceCatalog, classCode string) tui.InteractionAgent {
			return tui.NewResourcesWorkspaceAdapter(service, service, service, service, service, service, snapshot, classCode)
		}
		assistantAgent := tui.NewAssistantShellAgent()
		registry := domain.NewCatalogRegistry()
		catalogService := catalogo.NewServiceWithCatalogAuthority(postgres.NewCatalogAdminRepository(pool), registry, authority)
		catalogAgent := tui.NewCatalogAdminAdapter(catalogService, catalogService, catalogService, catalogService,
			catalogService, catalogService, catalogService, catalogService, catalogService, registry)
		model := tui.NewWithCatalogAuthority(tui.Handlers{
			Version: tui.Version(version),
			Config:  tui.Config(look),
			Status:  tui.Status(),
		}, assistantAgent, authority, agentFor, registry, catalogAgent)
		if _, err := launch(model).Run(); err != nil {
			fmt.Fprintf(errw, "TUI launcher failed: %v\n", err)
			return 1
		}
		return 0
	}

	switch args[0] {
	case "version":
		if len(args) != 1 {
			return usageError(errw, "version does not accept arguments")
		}
		fmt.Fprintln(out, version)
		return 0
	case "config":
		if len(args) != 2 || args[1] != "check" {
			return usageError(errw, "expected: config check")
		}
		cfg, err := config.Load(look)
		if err != nil {
			fmt.Fprintf(errw, "configuration is invalid: %v\n", err)
			return 1
		}
		fmt.Fprintf(out, "configuration is valid: host=%s port=%d name=%s user=%s sslmode=%s log_level=%s password=%s\n", cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBUser, cfg.DBSSLMode, cfg.LogLevel, cfg.DBPassword)
		return 0
	default:
		return usageError(errw, fmt.Sprintf("unknown command: %s", args[0]))
	}
}

func usageError(errw io.Writer, message string) int {
	fmt.Fprintln(errw, message)
	printUsage(errw)
	return 2
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: garfex [version | config check]")
}
