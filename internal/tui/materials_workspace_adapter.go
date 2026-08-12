package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// materialSearcher is the minimal surface the Materiales Maestros workspace
// needs from the application service to search the real catalog.
type materialSearcher interface {
	Search(ctx context.Context, criteria domain.SearchCriteria) ([]domain.Material, error)
}

// MaterialsWorkspaceAdapter is the production InteractionAgent for the
// Materiales Maestros workspace. It is a thin TUI-to-application-service
// adapter — not the Materials domain, not materiales.Service itself, and
// not a future LLM-driven agent. It never simulates a question; the only
// InteractionMessage kinds it ever returns are TextMessage, ErrorMessage and
// StructuredResult, driven entirely by the real Search result.
type MaterialsWorkspaceAdapter struct {
	materials materialSearcher
}

// NewMaterialsWorkspaceAdapter returns the production agent for the
// Materiales Maestros workspace. searcher is satisfied structurally by
// *materiales.Service — composed for real in cmd/garfex/main.go.
func NewMaterialsWorkspaceAdapter(searcher materialSearcher) *MaterialsWorkspaceAdapter {
	return &MaterialsWorkspaceAdapter{materials: searcher}
}

const materialsGreeting = "Materiales Maestros está conectado al catálogo real (PostgreSQL). Escribí un término para buscar."

// searchResultLimit requests one row beyond the visible page (10) so the
// adapter can tell "exactly 10 results" apart from "at least 11 results
// exist" without a separate count query.
const searchResultLimit = 11
const searchResultPageSize = 10

const moreResultsHint = "Hay más resultados — refiná tu búsqueda para acotar."

// notApplicableAttributeText mirrors internal/postgres's notApplicableState
// sentinel ("NOT_APPLICABLE"), the literal domain.MaterialAttributeValue.Text
// value the repository decodes onto a NOT_APPLICABLE attribute regardless of
// its declared Type. internal/domain does not expose an equivalent constant
// (its own NotApplicable field lives on AttributeRule, a catalog-rule
// concept, not on the runtime attribute value), and internal/tui must not
// import internal/postgres, so the literal is duplicated locally here.
const notApplicableAttributeText = "NOT_APPLICABLE"

// Greeting is shown once, before any user input, so the connection status
// is visible from the start of the workspace instead of only appearing
// after the user types something.
func (a *MaterialsWorkspaceAdapter) Greeting() InteractionMessage {
	return TextMessage{Text: materialsGreeting}
}

// Respond searches the real catalog for InputText input. It never simulates
// a question or a fabricated result: every message it returns is derived
// directly from what Search actually returned. Any other InteractionInput
// kind is unchanged from before this change — Search is not called.
func (a *MaterialsWorkspaceAdapter) Respond(ctx context.Context, input InteractionInput) (InteractionResponse, error) {
	if input.Kind != InputText {
		return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: materialsGreeting}}}, nil
	}

	criteria := domain.SearchCriteria{Text: input.Value, Limit: searchResultLimit}
	results, err := a.materials.Search(ctx, criteria)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude completar la búsqueda. Probá de nuevo en un momento."},
		}}, nil
	}

	if len(results) == 0 {
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: fmt.Sprintf("No encontré materiales que coincidan con %q.", input.Value)},
		}}, nil
	}

	hasMore := len(results) == searchResultLimit
	visible := results
	if hasMore {
		visible = results[:searchResultPageSize]
	}

	sections := make([]Section, len(visible))
	for i, material := range visible {
		sections[i] = materialSection(material)
	}

	result := StructuredResult{
		Title:    fmt.Sprintf("%d material(es) encontrado(s)", len(visible)),
		Sections: sections,
	}
	if hasMore {
		result.Subtitle = moreResultsHint
	}

	return InteractionResponse{Messages: []InteractionMessage{result}}, nil
}

// materialSection renders one search result using only real domain.Material
// fields — IdentityKey (a technical composite key) is deliberately never
// surfaced, mirroring renderMaterialDetail's existing commercial-field
// discipline in handlers.go. Unlike renderMaterialDetail, it also omits
// NOT_APPLICABLE attributes: a blank field is noise in a search result list,
// not information (renderMaterialDetail's single-material technical detail
// view is left as-is; this filtering is new and local to search results).
func materialSection(material domain.Material) Section {
	attributes := append([]domain.MaterialAttributeValue(nil), material.Attributes...)
	sort.SliceStable(attributes, func(i, j int) bool { return attributes[i].AttributeCode < attributes[j].AttributeCode })

	var headline []string
	fields := []Field{{Label: "Unidad natural", Value: material.NaturalUnit}}
	for _, attribute := range attributes {
		if attribute.Text == notApplicableAttributeText {
			continue
		}
		value := formatAttributeValue(attribute)
		headline = append(headline, value)
		fields = append(fields, Field{Label: attribute.AttributeCode, Value: value})
	}

	title := material.FamilyCode
	if len(headline) > 0 {
		title += " — " + strings.Join(headline, " · ")
	}
	return Section{Title: title, Fields: fields}
}
