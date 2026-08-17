package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// This file completes ResourcesWorkspaceAdapter (started in
// resource_editor.go by recursos-maestro PR6, which implemented only the
// create/edit/duplicate editor state machine) by adding the full workspace
// dispatch: Greeting/Respond, text search, the search-result detail view,
// and the lifecycle confirmation flow. It retires the older,
// Materials-only/unfiltered-only MaterialsWorkspaceAdapter
// (materials_workspace_adapter.go, deleted by this same PR) — every method
// below is that file's Resource-typed, classFilter-aware equivalent, so the
// SAME adapter type now serves both the unfiltered "/recursos" workspace
// (classFilter == "") and every class-scoped workspace like "/materiales"
// (classFilter == "MATERIAL") — one adapter TYPE, reused per workspace slot
// (recursos-maestro design D4), rather than one adapter per catalog.

const resourcesGreeting = "Recursos Maestros está conectado al catálogo real (PostgreSQL)."

// createResourceActionID is the Action.ID/InteractionInput.ActionID for the
// "Crear ..." entry in a resources-workspace-scoped "/" palette (see
// commands.go's materialsActions/workspaceActions and model.go's
// activePaletteActions). It starts the "nuevo recurso" create flow.
const createResourceActionID = "create-resource"

// searchResultLimit requests one row beyond the visible page (10) so the
// adapter can tell "exactly 10 results" apart from "at least 11 results
// exist" without a separate count query.
const searchResultLimit = 11
const searchResultPageSize = 10

// searchResultsKey identifies the selectable search-results QuestionRequest;
// selecting one of its Options opens that resource's detail view.
const searchResultsKey = "resources-search-results"

// resourcesDetailActionsKey identifies the ActionRequest offered after a
// detail view: Editar/Duplicar/lifecycle/volver a los resultados.
const resourcesDetailActionsKey = "resources-detail-actions"

// backActionID is the Action.ID/InteractionInput.ActionID for "volver a los
// resultados" from the detail view back to the search-results list.
const backActionID = "back"

// editActionID is the Action.ID/InteractionInput.ActionID for "Editar" from
// the detail view. It starts the "editar recurso" flow (startEditEditor,
// resource_editor.go).
const editActionID = "edit-resource"

// duplicateActionID is the Action.ID/InteractionInput.ActionID for
// "Duplicar" from the detail view (startDuplicateEditor, resource_editor.go).
const duplicateActionID = "duplicate-resource"

const (
	deactivateActionID        = "deactivate-resource"
	reactivateActionID        = "reactivate-resource"
	inactiveDiscoveryActionID = "discover-inactive-resources"
)

const resourcesLifecycleConfirmKey = "resources-lifecycle-confirm"

// notApplicableAttributeText mirrors internal/postgres's notApplicableState
// sentinel ("NOT_APPLICABLE"), the literal domain.ResourceAttributeValue.Text
// value the repository decodes onto a NOT_APPLICABLE attribute regardless of
// its declared Type. internal/domain does not expose an equivalent constant,
// and internal/tui must not import internal/postgres, so the literal is
// duplicated locally here — used both by this file and by resource_editor.go
// (attributePickerQuestion/resourcePresentation).
const notApplicableAttributeText = "NOT_APPLICABLE"

// Greeting is shown once, before any user input, so the connection status is
// visible from the start of the workspace instead of only appearing after
// the user types something.
func (a *ResourcesWorkspaceAdapter) Greeting() InteractionMessage {
	return TextMessage{Text: resourcesGreeting}
}

// Respond handles the interactions a resources workspace supports: a text
// search, selecting a search result to open its detail, "volver" back to the
// same result list, and the "nuevo recurso"/"Editar"/"Duplicar" flows (see
// resource_editor.go). An in-progress editor gets first refusal on any input
// keyed to it (or a cancellation); everything else falls through to the
// unchanged status/greeting fallback — Search/Get are not called.
func (a *ResourcesWorkspaceAdapter) Respond(ctx context.Context, input InteractionInput) (InteractionResponse, error) {
	if a.editor != nil {
		if response, handled := a.respondToEditor(ctx, input); handled {
			return response, nil
		}
	}
	switch {
	case input.Kind == InputAction && input.ActionID == createResourceActionID:
		return a.startCreateEditor()
	case input.Kind == InputText:
		a.lifecycleScope = domain.LifecycleScopeActive
		return a.searchResponse(ctx, input.Value, a.lifecycleScope)
	case input.Kind == InputAction && input.ActionID == searchResourcesActionID:
		a.lifecycleScope = domain.LifecycleScopeActive
		return InteractionResponse{}, nil
	case input.Kind == InputAction && input.ActionID == inactiveDiscoveryActionID:
		a.lifecycleScope = domain.LifecycleScopeInactive
		return a.searchResponse(ctx, a.lastQuery, a.lifecycleScope)
	case input.Kind == InputSelection && input.Key == searchResultsKey:
		return a.detailResponse(ctx, input.Value)
	case input.Kind == InputAction && input.ActionID == editActionID:
		return a.startEditEditor()
	case input.Kind == InputAction && input.ActionID == duplicateActionID:
		return a.startDuplicateEditor()
	case input.Kind == InputAction && input.ActionID == deactivateActionID:
		return a.startLifecycleConfirmation(false)
	case input.Kind == InputAction && input.ActionID == reactivateActionID:
		return a.startLifecycleConfirmation(true)
	case input.Kind == InputSelection && input.Key == resourcesLifecycleConfirmKey:
		return a.answerLifecycleConfirmation(ctx, input.Value)
	case input.Kind == InputAction && input.ActionID == backActionID:
		return a.searchResponse(ctx, a.lastQuery, a.lifecycleScope)
	}
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: resourcesGreeting}}}, nil
}

// searchResponse runs the real Search for text, scoped to a.classFilter (""
// leaves it unfiltered across every class — the unfiltered "/recursos"
// workspace; a non-empty class code narrows it — a class-scoped workspace
// like "/materiales"), and renders the result as a selectable
// QuestionRequest (or, for 0 results/errors, a plain TextMessage/
// ErrorMessage). It is shared between the initial InputText search and the
// "volver a los resultados" action, which re-runs the identical
// deterministic search instead of caching results.
func (a *ResourcesWorkspaceAdapter) searchResponse(ctx context.Context, text string, scope domain.ResourceLifecycleScope) (InteractionResponse, error) {
	criteria := domain.SearchCriteria{Text: text, ClassCode: a.classFilter, LifecycleScope: scope, Limit: searchResultLimit}
	results, err := a.resources.Search(ctx, criteria)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude completar la búsqueda. Probá de nuevo en un momento."},
		}}, nil
	}

	if len(results) == 0 {
		return InteractionResponse{Messages: []InteractionMessage{
			TextMessage{Text: fmt.Sprintf("No encontré recursos que coincidan con %q.", text)},
		}}, nil
	}

	hasMore := len(results) == searchResultLimit
	visible := results
	if hasMore {
		visible = results[:searchResultPageSize]
	}

	prompt := fmt.Sprintf("%d recurso(s) encontrado(s)", len(visible))
	if hasMore {
		prompt += " (hay más — refiná tu búsqueda para acotar)"
	}
	prompt += ":"

	options := make([]Option, len(visible))
	for i, resource := range visible {
		label := a.describer.Describe(resource)
		label += " · " + resourceStatus(resource)
		options[i] = Option{
			ID:    fmt.Sprintf("%d", i),
			Label: label,
			Value: resource.ClassCode + "|" + resource.IdentityKey,
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

// detailResponse opens the full detail of the resource encoded in value
// (ClassCode + "|" + IdentityKey, see resourcePresentation/searchResponse —
// design R1: Get's first argument is a CLASS code, never a family code) and
// offers Editar/Duplicar, the requested lifecycle transition, and "volver a los resultados".
func (a *ResourcesWorkspaceAdapter) detailResponse(ctx context.Context, value string) (InteractionResponse, error) {
	classCode, identityKey, ok := strings.Cut(value, "|")
	if !ok {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude abrir el detalle de ese recurso. Probá de nuevo en un momento."},
		}}, nil
	}

	resource, err := a.resourcesGetter.Get(ctx, classCode, identityKey)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude abrir el detalle de ese recurso. Probá de nuevo en un momento."},
		}}, nil
	}
	a.lastDetail = resource

	title, fields := a.resourcePresentation(resource)
	return InteractionResponse{
		Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}},
		Pending:  a.detailActionsRequest(),
	}, nil
}

// detailActionsRequest builds the "¿Qué querés hacer?" menu offered from a
// resource's detail view — shared by detailResponse (first opening it) and
// the "No, cancelar" path of the delete confirmation (returning to the same
// detail after declining).
func (a *ResourcesWorkspaceAdapter) detailActionsRequest() ActionRequest {
	active := resourceIsActive(a.lastDetail)
	lifecycleID, lifecycleLabel := reactivateActionID, "Reactivar"
	if active {
		lifecycleID, lifecycleLabel = deactivateActionID, "Desactivar"
	}
	return ActionRequest{
		ID:       resourcesDetailActionsKey,
		Key:      resourcesDetailActionsKey,
		Question: "¿Qué querés hacer?",
		Actions: []Action{
			{ID: editActionID, Label: "Editar", Value: editActionID, Target: ActionTargetAgent},
			{ID: duplicateActionID, Label: "Duplicar", Value: duplicateActionID, Target: ActionTargetAgent},
			{ID: lifecycleID, Label: lifecycleLabel, Value: lifecycleID, Target: ActionTargetAgent},
			{ID: backActionID, Label: "Volver a los resultados", Value: backActionID, Target: ActionTargetAgent},
		},
	}
}

func (a *ResourcesWorkspaceAdapter) startLifecycleConfirmation(reactivate bool) (InteractionResponse, error) {
	a.lifecycleTarget = reactivate
	title, _ := a.resourcePresentation(a.lastDetail)
	verb := "Desactivar"
	if reactivate {
		verb = "Reactivar"
	}
	question := fmt.Sprintf("%s recurso\n\n%s\n\n¿Seguro que querés %s este recurso?", verb, title, strings.ToLower(verb))
	return InteractionResponse{Pending: ConfirmationRequest{
		ID:           resourcesLifecycleConfirmKey,
		Key:          resourcesLifecycleConfirmKey,
		Question:     question,
		ConfirmLabel: "Sí, " + strings.ToLower(verb),
		CancelLabel:  "No, cancelar",
	}}, nil
}

// answerLifecycleConfirmation handles the confirmation's answer: "no" returns
// the user to the SAME detail view they were looking at (not a bare
// "cancelado" — they were just reading about a specific resource and said
// not to change it); "yes" performs the requested lifecycle transition or
// reports a generic failure without leaking the raw error.
func (a *ResourcesWorkspaceAdapter) answerLifecycleConfirmation(ctx context.Context, value string) (InteractionResponse, error) {
	if value != "yes" {
		title, fields := a.resourcePresentation(a.lastDetail)
		return InteractionResponse{
			Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}},
			Pending:  a.detailActionsRequest(),
		}, nil
	}
	reactivate := a.lifecycleTarget
	verb := "desactivar"
	transition := a.lifecycle.Deactivate
	if reactivate {
		verb, transition = "reactivar", a.lifecycle.Reactivate
	}
	result, err := transition(ctx, a.lastDetail.ID)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude " + verb + " el recurso. Probá de nuevo en un momento."},
		}}, nil
	}
	if result.Resource.ID == 0 || result.Resource.ClassCode == "" {
		result.Resource = a.lastDetail
	}
	result.Resource.Active = reactivate
	a.lastDetail = result.Resource
	title, fields := a.resourcePresentation(a.lastDetail)
	return InteractionResponse{Messages: []InteractionMessage{StructuredResult{Title: title, Fields: fields}}, Pending: a.detailActionsRequest()}, nil
}

func resourceIsActive(resource domain.Resource) bool { return resource.ID <= 0 || resource.Active }

func resourceStatus(resource domain.Resource) string {
	if resourceIsActive(resource) {
		return "Activo"
	}
	return "Inactivo"
}
