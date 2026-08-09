// Package tui provides the GARFEX terminal menu model and its injected handlers.
package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/config"
)

type Handler func() tea.Cmd

func Version(version string) Handler {
	return func() tea.Cmd { return func() tea.Msg { return resultMsg{text: version} } }
}

func Config(look func(string) (string, bool)) Handler {
	return func() tea.Cmd {
		return func() tea.Msg {
			cfg, err := config.Load(look)
			if err == nil {
				return resultMsg{text: "configuration is valid\npassword: " + cfg.DBPassword.String()}
			}
			var validation config.ValidationError
			if errors.As(err, &validation) {
				return resultMsg{err: fmt.Errorf("configuration is invalid: %s", validation.Var)}
			}
			return resultMsg{err: errors.New("configuration is invalid")}
		}
	}
}

func Status() Handler {
	return func() tea.Cmd {
		return func() tea.Msg {
			return resultMsg{text: "Version: available\nConfig check: available\nTUI menu: available"}
		}
	}
}
