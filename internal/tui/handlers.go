// Package tui provides the GARFEX terminal menu model and its injected handlers.
package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/config"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

type Handler func() tea.Cmd

// Result creates a TUI result message. It lets callers outside the tui package
// inject a result or an error into the model without exposing resultMsg.
func Result(text string, err error) tea.Msg {
	return resultMsg{text: text, err: err}
}

// Resources returns a handler that renders one resource's technical detail.
// getter is resourceGetter (defined in resource_editor.go: Get(ctx,
// classCode, identityKey) (domain.Resource, error)) — reused directly here
// rather than a second, duplicate interface declaration, since the shape is
// identical.
func Resources(getter resourceGetter, classCode, identityKey string) Handler {
	return func() tea.Cmd {
		return func() tea.Msg {
			resource, err := getter.Get(context.Background(), classCode, identityKey)
			if err != nil {
				return resultMsg{err: fmt.Errorf("recursos: %w", err)}
			}
			return resultMsg{text: renderResourceDetail(resource)}
		}
	}
}

func renderResourceDetail(resource domain.Resource) string {
	attributes := append([]domain.ResourceAttributeValue(nil), resource.Attributes...)
	sort.SliceStable(attributes, func(i, j int) bool {
		return attributes[i].AttributeCode < attributes[j].AttributeCode
	})

	var b strings.Builder
	fmt.Fprintf(&b, "Recurso\nUnidad natural: %s\nAtributos técnicos:\n", resource.NaturalUnit)
	for _, attribute := range attributes {
		// formatResourceAttributeValue is defined in resource_editor.go —
		// reused directly rather than duplicated here, since it already
		// operates on the same domain.ResourceAttributeValue shape.
		fmt.Fprintf(&b, "- %s: %s\n", attribute.AttributeCode, formatResourceAttributeValue(attribute))
	}
	return b.String()
}

// ErrorHandler returns a handler that presents err as a result message. It is
// used when a dependency cannot be built and the failure must be surfaced in
// the TUI instead of aborting the program.
func ErrorHandler(err error) Handler {
	return func() tea.Cmd {
		return func() tea.Msg {
			return resultMsg{err: err}
		}
	}
}

func Version(version string) Handler {
	return func() tea.Cmd { return func() tea.Msg { return resultMsg{text: version} } }
}

func Config(look func(string) (string, bool)) Handler {
	return func() tea.Cmd {
		return func() tea.Msg {
			cfg, err := config.Load(look)
			if err == nil {
				return resultMsg{text: "configuración válida\ncontraseña: " + cfg.DBPassword.String()}
			}
			var validation config.ValidationError
			if errors.As(err, &validation) {
				return resultMsg{err: fmt.Errorf("configuración inválida: %s", validation.Var)}
			}
			return resultMsg{err: errors.New("configuración inválida")}
		}
	}
}

func Status() Handler {
	return func() tea.Cmd {
		return func() tea.Msg {
			return resultMsg{text: "Versión: disponible\nVerificación de configuración: disponible\nMenú TUI: disponible"}
		}
	}
}
