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
// not a future LLM-driven agent. It never simulates a question; every
// InteractionMessage/Pending it returns is driven entirely by the real
// Search/Get results.
type MaterialsWorkspaceAdapter struct {
	materials       materialSearcher
	materialsGetter materialGetter
	// lastQuery remembers the text of the most recent successful search so
	// the "volver a los resultados" action can reproduce the identical
	// result list deterministically, without a separate cache of results.
	lastQuery string
}

// NewMaterialsWorkspaceAdapter returns the production agent for the
// Materiales Maestros workspace. searcher and getter are satisfied
// structurally by *materiales.Service — composed for real in
// cmd/garfex/main.go.
func NewMaterialsWorkspaceAdapter(searcher materialSearcher, getter materialGetter) *MaterialsWorkspaceAdapter {
	return &MaterialsWorkspaceAdapter{materials: searcher, materialsGetter: getter}
}

const materialsGreeting = "Materiales Maestros está conectado al catálogo real (PostgreSQL). Escribí un término para buscar."

// searchResultLimit requests one row beyond the visible page (10) so the
// adapter can tell "exactly 10 results" apart from "at least 11 results
// exist" without a separate count query.
const searchResultLimit = 11
const searchResultPageSize = 10

// searchResultsKey identifies the selectable search-results QuestionRequest;
// selecting one of its Options opens that material's detail view.
const searchResultsKey = "materials-search-results"

// materialsDetailActionsKey identifies the ActionRequest offered after a
// detail view, currently only "volver a los resultados".
const materialsDetailActionsKey = "materials-detail-actions"

// backActionID is the Action.ID/InteractionInput.ActionID for "volver a los
// resultados" from the detail view back to the search-results list.
const backActionID = "back"

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

// Respond handles the three interactions this workspace supports: a text
// search, selecting a search result to open its detail, and "volver" back
// to the same result list. Any other InteractionInput falls through to the
// unchanged status/greeting fallback — Search/Get are not called.
func (a *MaterialsWorkspaceAdapter) Respond(ctx context.Context, input InteractionInput) (InteractionResponse, error) {
	switch {
	case input.Kind == InputText:
		return a.searchResponse(ctx, input.Value)
	case input.Kind == InputSelection && input.Key == searchResultsKey:
		return a.detailResponse(ctx, input.Value)
	case input.Kind == InputAction && input.ActionID == backActionID:
		return a.searchResponse(ctx, a.lastQuery)
	}
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: materialsGreeting}}}, nil
}

// searchResponse runs the real Search for text and renders the result as a
// selectable QuestionRequest (or, for 0 results/errors, the same plain
// TextMessage/ErrorMessage as before this change). It is shared between the
// initial InputText search and the "volver a los resultados" action, which
// re-runs the identical deterministic search instead of caching results.
func (a *MaterialsWorkspaceAdapter) searchResponse(ctx context.Context, text string) (InteractionResponse, error) {
	criteria := domain.SearchCriteria{Text: text, Limit: searchResultLimit}
	results, err := a.materials.Search(ctx, criteria)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude completar la búsqueda. Probá de nuevo en un momento."},
		}}, nil
	}

	if len(results) == 0 {
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: fmt.Sprintf("No encontré materiales que coincidan con %q.", text)},
		}}, nil
	}

	hasMore := len(results) == searchResultLimit
	visible := results
	if hasMore {
		visible = results[:searchResultPageSize]
	}

	prompt := fmt.Sprintf("%d material(es) encontrado(s)", len(visible))
	if hasMore {
		prompt += " (hay más — refiná tu búsqueda para acotar)"
	}
	prompt += ":"

	options := make([]Option, len(visible))
	for i, material := range visible {
		title, _ := materialPresentation(material)
		options[i] = Option{
			ID:    fmt.Sprintf("%d", i),
			Label: title,
			Value: material.FamilyCode + "|" + material.IdentityKey,
		}
	}

	a.lastQuery = text

	return InteractionResponse{Pending: QuestionRequest{
		ID:            searchResultsKey,
		Key:           searchResultsKey,
		Prompt:        prompt,
		Question:      prompt,
		SelectionMode: SelectionSingle,
		Options:       options,
	}}, nil
}

// detailResponse opens the full detail of the material encoded in value
// (FamilyCode + "|" + IdentityKey, see materialPresentation/searchResponse) and
// offers "volver a los resultados" to return to the same search.
func (a *MaterialsWorkspaceAdapter) detailResponse(ctx context.Context, value string) (InteractionResponse, error) {
	familyCode, identityKey, ok := strings.Cut(value, "|")
	if !ok {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude abrir el detalle de ese material. Probá de nuevo en un momento."},
		}}, nil
	}

	material, err := a.materialsGetter.Get(ctx, familyCode, identityKey)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude abrir el detalle de ese material. Probá de nuevo en un momento."},
		}}, nil
	}

	title, fields := materialPresentation(material)
	return InteractionResponse{
		Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}},
		Pending: ActionRequest{
			ID:       materialsDetailActionsKey,
			Key:      materialsDetailActionsKey,
			Question: "¿Qué querés hacer?",
			Actions: []Action{
				{ID: backActionID, Label: "Volver a los resultados", Value: backActionID, Target: ActionTargetAgent},
			},
		},
	}, nil
}

// materialPresentation builds the shared "one material" presentation used both
// as a compact selectable-list Option.Label (searchResponse) and as the
// detail view's own Title/Fields (detailResponse) — built once, reused
// twice. IdentityKey (a technical composite key) is deliberately never
// surfaced, mirroring renderMaterialDetail's existing commercial-field
// discipline in handlers.go. NOT_APPLICABLE attributes are omitted: a blank
// field is noise, not information.
func materialPresentation(material domain.Material) (title string, fields []Field) {
	attributes := append([]domain.MaterialAttributeValue(nil), material.Attributes...)
	sort.SliceStable(attributes, func(i, j int) bool { return attributes[i].AttributeCode < attributes[j].AttributeCode })

	var headline []string
	fields = []Field{{Label: "Unidad natural", Value: material.NaturalUnit}}
	for _, attribute := range attributes {
		if attribute.Text == notApplicableAttributeText {
			continue
		}
		value := formatAttributeValue(attribute)
		headline = append(headline, value)
		fields = append(fields, Field{Label: attribute.AttributeCode, Value: value})
	}

	title = material.FamilyCode
	if len(headline) > 0 {
		title += " — " + strings.Join(headline, " · ")
	}
	return title, fields
}
