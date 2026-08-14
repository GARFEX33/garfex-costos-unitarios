package main

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
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

// repositoryBuilder builds the real Resources repository from a DSN. It is
// injected so run() is unit-testable without a real Postgres instance.
type repositoryBuilder func(ctx context.Context, dsn string) (domain.ResourceRepository, error)

// catalogBuilder builds the domain.ResourceCatalog run() validates and wires
// through the rest of the composition. It is injected (defaulting to
// domain.NewResourceCatalog in main()) so a deliberately-broken catalog can
// drive the Validate() fail-fast path in a test without touching the real
// seed data (recursos-maestro design §8's structural guard).
type catalogBuilder func() domain.ResourceCatalog

func main() {
	os.Exit(run(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, newProgram, newPostgresRepository, domain.NewResourceCatalog))
}

func newProgram(model tea.Model) program { return tea.NewProgram(model) }

func newPostgresRepository(ctx context.Context, dsn string) (domain.ResourceRepository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return postgres.NewResourceRepository(pool), nil
}

func run(args []string, look func(string) (string, bool), out, errw io.Writer, launch programLauncher, buildRepository repositoryBuilder, buildCatalog catalogBuilder) int {
	if len(args) == 0 {
		cfg, err := config.Load(look)
		if err != nil {
			fmt.Fprintf(errw, "configuration is invalid: %v\n", err)
			return 1
		}
		// Fail fast: a structurally malformed catalog (e.g. a future data-only
		// class addition with a dangling reference) must never reach the TUI
		// silently misbehaving — it aborts the launch instead (design §8).
		catalog := buildCatalog()
		if err := catalog.Validate(); err != nil {
			fmt.Fprintf(errw, "catálogo de recursos inválido: %v\n", err)
			return 1
		}
		repo, err := buildRepository(context.Background(), cfg.DSN())
		if err != nil {
			fmt.Fprintf(errw, "database unavailable: %v\n", err)
			return 1
		}
		service := recursos.NewService(repo, catalog)
		agentFor := func(classCode string) tui.InteractionAgent {
			return tui.NewResourcesWorkspaceAdapter(service, service, service, service, service, service, catalog, classCode)
		}
		assistantAgent := tui.NewAssistantShellAgent()
		model := tui.NewWithCatalog(tui.Handlers{
			Version: tui.Version(version),
			Config:  tui.Config(look),
			Status:  tui.Status(),
		}, assistantAgent, catalog, agentFor)
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
