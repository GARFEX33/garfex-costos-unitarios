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

// materialLister is the minimal surface the materials handler needs from the
// application service.
type materialGetter interface {
	Get(ctx context.Context, familyCode, identityKey string) (domain.Material, error)
}

// Materials returns a handler that renders one material's technical detail.
func Materials(getter materialGetter, familyCode, identityKey string) Handler {
	return func() tea.Cmd {
		return func() tea.Msg {
			material, err := getter.Get(context.Background(), familyCode, identityKey)
			if err != nil {
				return resultMsg{err: fmt.Errorf("materiales: %w", err)}
			}
			return resultMsg{text: renderMaterialDetail(material)}
		}
	}
}

func renderMaterialDetail(material domain.Material) string {
	attributes := append([]domain.MaterialAttributeValue(nil), material.Attributes...)
	sort.SliceStable(attributes, func(i, j int) bool {
		return attributes[i].AttributeCode < attributes[j].AttributeCode
	})

	var b strings.Builder
	fmt.Fprintf(&b, "Material\nUnidad natural: %s\nAtributos técnicos:\n", material.NaturalUnit)
	for _, attribute := range attributes {
		fmt.Fprintf(&b, "- %s: %s\n", attribute.AttributeCode, formatAttributeValue(attribute))
	}
	return b.String()
}

func formatAttributeValue(attribute domain.MaterialAttributeValue) string {
	switch attribute.Type {
	case domain.ValueTypeControlledOption:
		return attribute.OptionCode
	case domain.ValueTypeInteger:
		if attribute.Integer != nil {
			return fmt.Sprintf("%d", *attribute.Integer)
		}
	case domain.ValueTypeDecimal:
		if attribute.Decimal != nil {
			return attribute.Decimal.String()
		}
	case domain.ValueTypeQuantity:
		if attribute.Quantity != nil {
			return attribute.Quantity.Value.String() + " " + attribute.Quantity.UnitCode
		}
	case domain.ValueTypeBoolean:
		if attribute.Boolean != nil {
			return fmt.Sprintf("%t", *attribute.Boolean)
		}
	case domain.ValueTypeControlledText:
		return attribute.Text
	}
	return ""
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
