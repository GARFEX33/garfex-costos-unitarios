package main

import (
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/config"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/tui"
)

var version = "dev"

type program interface {
	Run() (tea.Model, error)
}

type programLauncher func(tea.Model) program

func main() {
	os.Exit(run(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, newProgram))
}

func newProgram(model tea.Model) program { return tea.NewProgram(model) }

func run(args []string, look func(string) (string, bool), out, errw io.Writer, launch programLauncher) int {
	if len(args) == 0 {
		if _, err := launch(tui.New(tui.Handlers{
			Version: tui.Version(version),
			Config:  tui.Config(look),
			Status:  tui.Status(),
		})).Run(); err != nil {
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
