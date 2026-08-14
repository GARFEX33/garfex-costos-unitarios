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

// materialDescriber is the minimal surface the Materiales Maestros workspace
// needs to resolve a material's canonical presentation — deliberately kept
// separate from materialSearcher/materialGetter so the adapter never needs
// to know about domain.MaterialsCatalog directly.
type materialDescriber interface {
	Describe(material domain.Material) string
}

// materialCreator is the minimal surface the generic MaterialEditor needs to
// persist a newly-built candidate Material.
type materialCreator interface {
	Create(ctx context.Context, material domain.Material) error
}

// materialUpdater is the minimal surface the generic MaterialEditor needs to
// persist changes to an existing Material (identified by its stable ID,
// independent of IdentityKey, which editing can change — see Material.ID).
type materialUpdater interface {
	Update(ctx context.Context, material domain.Material) error
}

// materialDeleter is the minimal surface the "Eliminar" flow needs to soft-
// delete a material by its stable ID (never a hard delete — see
// materiales.Service.Delete/MaterialRepository.SetActive, already built and
// integration-tested in an earlier PR).
type materialDeleter interface {
	Delete(ctx context.Context, id int64) error
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
	describer       materialDescriber
	creator         materialCreator
	updater         materialUpdater
	deleter         materialDeleter
	// lastQuery remembers the text of the most recent successful search so
	// the "volver a los resultados" action can reproduce the identical
	// result list deterministically, without a separate cache of results.
	lastQuery string
	// lastDetail remembers the material currently shown in the detail view
	// so a later "Editar" selection knows which one to edit — mirrors how
	// lastQuery already remembers the last search text for "volver".
	lastDetail domain.Material
	// editor holds the in-progress "nuevo material" create/edit flow, if
	// any; nil means no create/edit flow is in progress. See
	// material_editor.go.
	editor *materialEditorState
}

// NewMaterialsWorkspaceAdapter returns the production agent for the
// Materiales Maestros workspace. searcher, getter, describer, creator,
// updater and deleter are satisfied structurally by *materiales.Service —
// composed for real in cmd/garfex/main.go.
func NewMaterialsWorkspaceAdapter(searcher materialSearcher, getter materialGetter, describer materialDescriber, creator materialCreator, updater materialUpdater, deleter materialDeleter) *MaterialsWorkspaceAdapter {
	return &MaterialsWorkspaceAdapter{materials: searcher, materialsGetter: getter, describer: describer, creator: creator, updater: updater, deleter: deleter}
}

const materialsGreeting = "Materiales Maestros está conectado al catálogo real (PostgreSQL). Buscá un material o usá / para acciones."

// createMaterialActionID is the Action.ID/InteractionInput.ActionID for the
// "Crear material" entry in the materials-workspace-scoped "/" palette (see
// commands.go's materialsActions and model.go's activePaletteActions). It
// starts the "nuevo material" create flow — the free-text trigger this
// replaced is gone entirely, per the project owner's explicit "no keywords
// mágicas escondidas dentro de la búsqueda" direction.
const createMaterialActionID = "create-material"

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

// editActionID is the Action.ID/InteractionInput.ActionID for "Editar" from
// the detail view. It starts the "editar material" flow (see
// startEditEditor in material_editor.go), reusing the same MaterialEditor
// engine as "Crear material" but entered partway through, at the first
// applicable attribute, since Family/ProductType are fixed during edit.
const editActionID = "edit-material"

// duplicateActionID is the Action.ID/InteractionInput.ActionID for
// "Duplicar" from the detail view. It starts the "duplicar material" flow
// (see startDuplicateEditor in material_editor.go), reusing the same
// field-picker loop "Editar" uses, pre-loaded with the source material's
// current values, but persisting via Create (a brand-new Material) rather
// than Update.
const duplicateActionID = "duplicate-material"

// deleteActionID is the Action.ID/InteractionInput.ActionID for "Eliminar"
// from the detail view. It shows a confirmation naming the material before
// doing anything — see startDeleteConfirmation/answerDeleteConfirmation.
const deleteActionID = "delete-material"

// materialsDeleteConfirmKey identifies the ConfirmationRequest shown before
// soft-deleting a material.
const materialsDeleteConfirmKey = "materials-delete-confirm"

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

// Respond handles the interactions this workspace supports: a text search,
// selecting a search result to open its detail, "volver" back to the same
// result list, and the "nuevo material"/"Editar"/"Duplicar" flows (see
// material_editor.go). An in-progress editor gets first refusal on any input
// keyed to it (or a cancellation); everything else falls through to the
// unchanged status/greeting fallback — Search/Get are not called.
func (a *MaterialsWorkspaceAdapter) Respond(ctx context.Context, input InteractionInput) (InteractionResponse, error) {
	if a.editor != nil {
		if response, handled := a.respondToEditor(ctx, input); handled {
			return response, nil
		}
	}
	switch {
	case input.Kind == InputAction && input.ActionID == createMaterialActionID:
		return a.startCreateEditor()
	case input.Kind == InputText:
		return a.searchResponse(ctx, input.Value)
	case input.Kind == InputSelection && input.Key == searchResultsKey:
		return a.detailResponse(ctx, input.Value)
	case input.Kind == InputAction && input.ActionID == editActionID:
		return a.startEditEditor()
	case input.Kind == InputAction && input.ActionID == duplicateActionID:
		return a.startDuplicateEditor()
	case input.Kind == InputAction && input.ActionID == deleteActionID:
		return a.startDeleteConfirmation()
	case input.Kind == InputSelection && input.Key == materialsDeleteConfirmKey:
		return a.answerDeleteConfirmation(ctx, input.Value)
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
		title, _ := a.materialPresentation(material)
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
	a.lastDetail = material

	title, fields := a.materialPresentation(material)
	return InteractionResponse{
		Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}},
		Pending:  detailActionsRequest(),
	}, nil
}

// detailActionsRequest builds the "¿Qué querés hacer?" menu offered from a
// material's detail view — shared by detailResponse (first opening it) and
// the "No, cancelar" path of the delete confirmation (returning to the same
// detail after declining).
func detailActionsRequest() ActionRequest {
	return ActionRequest{
		ID:       materialsDetailActionsKey,
		Key:      materialsDetailActionsKey,
		Question: "¿Qué querés hacer?",
		Actions: []Action{
			{ID: editActionID, Label: "Editar", Value: editActionID, Target: ActionTargetAgent},
			{ID: duplicateActionID, Label: "Duplicar", Value: duplicateActionID, Target: ActionTargetAgent},
			{ID: deleteActionID, Label: "Eliminar", Value: deleteActionID, Target: ActionTargetAgent},
			{ID: backActionID, Label: "Volver a los resultados", Value: backActionID, Target: ActionTargetAgent},
		},
	}
}

// startDeleteConfirmation shows a confirmation naming the material currently
// in the detail view (a.lastDetail) before doing anything destructive —
// nothing is deleted yet at this point.
func (a *MaterialsWorkspaceAdapter) startDeleteConfirmation() (InteractionResponse, error) {
	title, _ := a.materialPresentation(a.lastDetail)
	question := fmt.Sprintf("Eliminar material\n\n%s\n\n¿Seguro que querés eliminar este material?", title)
	return InteractionResponse{Pending: ConfirmationRequest{
		ID:           materialsDeleteConfirmKey,
		Key:          materialsDeleteConfirmKey,
		Question:     question,
		ConfirmLabel: "Sí, eliminar",
		CancelLabel:  "No, cancelar",
	}}, nil
}

// answerDeleteConfirmation handles the confirmation's answer: "no" returns
// the user to the SAME detail view they were looking at (not a bare
// "cancelado" — they were just reading about a specific material and said
// not to delete it, so landing back there is the natural result); "yes"
// soft-deletes it (materialDeleter.Delete — never a hard delete) and
// confirms what happened, or reports a generic failure without leaking the
// raw error, matching this file's existing discipline elsewhere.
func (a *MaterialsWorkspaceAdapter) answerDeleteConfirmation(ctx context.Context, value string) (InteractionResponse, error) {
	if value != "yes" {
		title, fields := a.materialPresentation(a.lastDetail)
		return InteractionResponse{
			Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}},
			Pending:  detailActionsRequest(),
		}, nil
	}
	title, _ := a.materialPresentation(a.lastDetail)
	if err := a.deleter.Delete(ctx, a.lastDetail.ID); err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude eliminar el material. Probá de nuevo en un momento."},
		}}, nil
	}
	return InteractionResponse{Messages: []InteractionMessage{
		TextMessage{Text: "Material eliminado: " + title},
	}}, nil
}

// materialPresentation builds the shared "one material" presentation used both
// as a compact selectable-list Option.Label (searchResponse) and as the
// detail view's own Title/Fields (detailResponse) — built once, reused
// twice. IdentityKey (a technical composite key) is deliberately never
// surfaced, mirroring renderMaterialDetail's existing commercial-field
// discipline in handlers.go. NOT_APPLICABLE attributes are omitted: a blank
// field is noise, not information.
//
// The title is the catalog-controlled canonical presentation resolved via
// describer (see materialDescriber): it is never an automatic dump of every
// attribute, and it never falls back to one when a ProductType has no
// presentation configured (a.describer.Describe returning "" simply leaves
// the title as the bare FamilyCode). The technical detail fields loop below
// is unchanged by this — same attributes, same order, same discipline.
func (a *MaterialsWorkspaceAdapter) materialPresentation(material domain.Material) (title string, fields []Field) {
	attributes := append([]domain.MaterialAttributeValue(nil), material.Attributes...)
	sort.SliceStable(attributes, func(i, j int) bool { return attributes[i].AttributeCode < attributes[j].AttributeCode })

	fields = []Field{{Label: "Unidad natural", Value: material.NaturalUnit}}
	for _, attribute := range attributes {
		if attribute.Text == notApplicableAttributeText {
			continue
		}
		fields = append(fields, Field{Label: attribute.AttributeCode, Value: formatAttributeValue(attribute)})
	}

	title = material.FamilyCode
	if presentation := a.describer.Describe(material); presentation != "" {
		title += " — " + presentation
	}
	return title, fields
}
