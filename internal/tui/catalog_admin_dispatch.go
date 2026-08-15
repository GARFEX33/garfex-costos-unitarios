package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// catalogAdminGreeting is shown once when the "Configuración" workspace is
// first entered — mirrors resources_workspace_dispatch.go's
// resourcesGreeting.
const catalogAdminGreeting = "Configuración del catálogo de recursos. Usá / para elegir qué administrar."

// catalogOpenActionPrefix namespaces every "abrir <kind>" palette leaf
// action id (see commands.go's buildCatalogAdminActions) — routing is a
// prefix-strip + CatalogKindCode lookup, not a per-kind switch (spec:
// Generic Descriptor-Driven Engine).
const catalogOpenActionPrefix = "catalog-open:"

// catalogIdentityActionID is the "Identidad" palette leaf's action id
// (design §8's business-ask menu shape lists it alongside Aplicabilidad and
// Presentación under "Configuración de tipos") — a TUI-layer-only routing
// sentinel, since Identidad is not its own CatalogKind (design
// reconciliation: identity_participates is a field on KindAttributeBinding,
// not a separate kind). It opens the exact same KindAttributeBinding flow as
// the "Aplicabilidad" leaf; PR7 adds the Identidad-change warning on top
// (explicitly out of this PR's scope).
const catalogIdentityActionID = "catalog-open:IDENTIDAD"

// catalogKindMenuKey identifies the searchable "existing record or crear
// nueva/o" QuestionRequest a kind's palette leaf opens (startKindMenu).
const catalogKindMenuKey = "catalog-admin-kind-menu"

// catalogCreateNewOptionID is the Option.ID/Value startKindMenu always lists
// first — selecting it starts startCreateFlow for the active kind.
const catalogCreateNewOptionID = "catalog-admin-create-new"

// catalogRecordActionsKey identifies the ActionRequest offered after opening
// an existing record from startKindMenu (openRecordDetail, task 7.1) —
// Editar / Desactivar-or-Reactivar / Eliminar / Volver — mirrors
// resources_workspace_dispatch.go's resourcesDetailActionsKey/
// detailActionsRequest precedent (recursos-maestro): a record detail view
// followed by a lifecycle-actions menu, rather than jumping straight into
// edit.
const catalogRecordActionsKey = "catalog-admin-record-actions"

const catalogRecordEditActionID = "catalog-record-edit"
const catalogRecordDeactivateActionID = "catalog-record-deactivate"
const catalogRecordReactivateActionID = "catalog-record-reactivate"
const catalogRecordDeleteActionID = "catalog-record-delete"
const catalogRecordBackActionID = "catalog-record-back"

// catalogDeleteConfirmKey identifies the ConfirmationRequest shown before a
// real (unguarded) hard delete — mirrors resources_workspace_dispatch.go's
// resourcesDeleteConfirmKey/startDeleteConfirmation/answerDeleteConfirmation
// precedent.
const catalogDeleteConfirmKey = "catalog-admin-delete-confirm"

func catalogOpenActionID(kind domain.CatalogKindCode) string {
	return catalogOpenActionPrefix + string(kind)
}

// CatalogAdminAdapter is the production InteractionAgent for the single
// "Configuración" workspace (design D13/§8): ONE adapter instance serves
// EVERY registered CatalogKind through the SAME generic engine
// (catalog_admin.go), differing only by descriptor data — never a per-kind
// adapter, unlike the per-class ResourcesWorkspaceAdapter instances (design
// D4 there is about per-class *instances* of one type; here there is
// exactly one instance total, since catalog *structure* is not scoped by
// resource class the way resource *instances* are).
type CatalogAdminAdapter struct {
	lister            catalogLister
	getter            catalogGetter
	creator           catalogRecordCreator
	updater           catalogRecordUpdater
	dependencyChecker catalogDependencyChecker
	referenceChecker  catalogReferenceChecker
	deactivator       catalogDeactivator
	reactivator       catalogReactivator
	deleter           catalogDeleter
	registry          domain.CatalogRegistry
	// activeKind is the kind whose "existing record or crear nueva/o" menu
	// (startKindMenu) is currently pending, "" when none is.
	activeKind domain.CatalogKindCode
	// activeRecord is the record currently shown in the detail/lifecycle-
	// actions view (openRecordDetail/recordDetailResponse, task 7.1) — the
	// catalog-admin counterpart of ResourcesWorkspaceAdapter.lastDetail.
	activeRecord domain.CatalogRecord
	// editor holds the in-progress create/edit flow, if any; nil means none
	// is in progress. See catalog_admin.go.
	editor *catalogEditorState
}

// NewCatalogAdminAdapter returns a CatalogAdminAdapter backed by lister/
// getter/creator/updater/dependencyChecker/referenceChecker/deactivator/
// reactivator/deleter (satisfied structurally by *catalogo.Service in
// production — composed once in cmd/garfex/main.go, see the recursos
// analogue there) and registry (the fixed set of administrable CatalogKinds,
// design §3).
func NewCatalogAdminAdapter(
	lister catalogLister,
	getter catalogGetter,
	creator catalogRecordCreator,
	updater catalogRecordUpdater,
	dependencyChecker catalogDependencyChecker,
	referenceChecker catalogReferenceChecker,
	deactivator catalogDeactivator,
	reactivator catalogReactivator,
	deleter catalogDeleter,
	registry domain.CatalogRegistry,
) *CatalogAdminAdapter {
	return &CatalogAdminAdapter{
		lister: lister, getter: getter, creator: creator, updater: updater,
		dependencyChecker: dependencyChecker, referenceChecker: referenceChecker,
		deactivator: deactivator, reactivator: reactivator, deleter: deleter,
		registry: registry,
	}
}

// Greeting is shown once when the "Configuración" workspace is first
// entered.
func (a *CatalogAdminAdapter) Greeting() InteractionMessage {
	return TextMessage{Text: catalogAdminGreeting}
}

// Respond handles the interactions the "Configuración" workspace supports:
// opening one registered kind's menu (via its palette leaf action, see
// commands.go's buildCatalogAdminActions), picking "crear nueva/o" or an
// existing record from that menu, and the in-progress create/edit flow
// itself (see catalog_admin.go). An in-progress editor gets first refusal on
// any input keyed to it (or a cancellation); everything else falls through
// to the unchanged greeting fallback.
func (a *CatalogAdminAdapter) Respond(ctx context.Context, input InteractionInput) (InteractionResponse, error) {
	if a.editor != nil {
		if response, handled := a.respondToEditor(ctx, input); handled {
			return response, nil
		}
	}
	switch {
	case input.Kind == InputAction && input.ActionID == catalogIdentityActionID:
		return a.startKindMenu(ctx, domain.KindAttributeBinding)
	case input.Kind == InputAction && strings.HasPrefix(input.ActionID, catalogOpenActionPrefix):
		kind := domain.CatalogKindCode(strings.TrimPrefix(input.ActionID, catalogOpenActionPrefix))
		return a.startKindMenu(ctx, kind)
	case input.Kind == InputSelection && input.Key == catalogKindMenuKey:
		if input.Value == catalogCreateNewOptionID {
			return a.startCreateFlow(ctx, a.activeKind)
		}
		return a.openRecordDetail(ctx, a.activeKind, input.Value)
	case input.Kind == InputAction && input.ActionID == catalogRecordEditActionID:
		return a.startEditFlow(ctx, a.activeKind, a.activeRecord)
	case input.Kind == InputAction && input.ActionID == catalogRecordDeactivateActionID:
		return a.answerSetActive(ctx, false)
	case input.Kind == InputAction && input.ActionID == catalogRecordReactivateActionID:
		return a.answerSetActive(ctx, true)
	case input.Kind == InputAction && input.ActionID == catalogRecordDeleteActionID:
		return a.startDeleteConfirmation(ctx)
	case input.Kind == InputSelection && input.Key == catalogDeleteConfirmKey:
		return a.answerDeleteConfirmation(ctx, input.Value)
	case input.Kind == InputAction && input.ActionID == catalogRecordBackActionID:
		return a.startKindMenu(ctx, a.activeKind)
	}
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: catalogAdminGreeting}}}, nil
}

// startKindMenu opens kind's "existing record or crear nueva/o" menu: a
// SelectionSearchable QuestionRequest whose Options are "+ Crear nueva/o
// <Singular>" first, then every existing record (search-before-create, D12
// spirit, reusing model.go's already-tested SelectionSearchable
// type-to-filter behavior instead of a separate free-text search step).
func (a *CatalogAdminAdapter) startKindMenu(ctx context.Context, kind domain.CatalogKindCode) (InteractionResponse, error) {
	def, ok := a.registry.Kind(kind)
	if !ok {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No reconozco ese tipo de catálogo."},
		}}, nil
	}
	records, err := a.lister.List(ctx, kind, domain.CatalogFilter{})
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude cargar el listado. Probá de nuevo en un momento."},
		}}, nil
	}
	a.activeKind = kind

	options := make([]Option, 0, len(records)+1)
	options = append(options, Option{ID: catalogCreateNewOptionID, Label: "+ Crear nueva/o " + def.Singular, Value: catalogCreateNewOptionID})
	for _, rec := range records {
		id := fmt.Sprintf("%d", rec.ID)
		options = append(options, Option{ID: id, Label: catalogRecordDisplayLabel(def, rec), Value: id})
	}

	prompt := fmt.Sprintf("%s — elegí una/o existente o creá nueva/o", def.Plural)
	return InteractionResponse{Pending: QuestionRequest{
		ID: catalogKindMenuKey, Key: catalogKindMenuKey, Prompt: prompt, Question: prompt,
		SelectionMode: SelectionSearchable, Options: options,
	}}, nil
}

// openRecordForEdit resolves idText (a startKindMenu Option.Value) to a real
// record via getter.Get and starts startEditFlow for it.
func (a *CatalogAdminAdapter) openRecordForEdit(ctx context.Context, kind domain.CatalogKindCode, idText string) (InteractionResponse, error) {
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude abrir ese registro."},
		}}, nil
	}
	rec, err := a.getter.Get(ctx, kind, id)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude abrir ese registro. Probá de nuevo en un momento."},
		}}, nil
	}
	return a.startEditFlow(ctx, kind, rec)
}

// openRecordDetail resolves idText (a startKindMenu Option.Value) to a real
// record and opens its detail/lifecycle-actions view (task 7.1) — the
// production dispatch path for picking an existing record, replacing PR6's
// direct-to-edit shortcut (openRecordForEdit, kept above for the low-level
// edit-flow tests that still call it directly).
func (a *CatalogAdminAdapter) openRecordDetail(ctx context.Context, kind domain.CatalogKindCode, idText string) (InteractionResponse, error) {
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude abrir ese registro."},
		}}, nil
	}
	rec, err := a.getter.Get(ctx, kind, id)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude abrir ese registro. Probá de nuevo en un momento."},
		}}, nil
	}
	a.activeKind = kind
	a.activeRecord = rec
	return a.recordDetailResponse(), nil
}

// recordDetailResponse renders a.activeRecord's detail (StructuredResult)
// plus its lifecycle-actions menu — shared by openRecordDetail (first
// opening it) and every lifecycle action that returns to the same view
// (Desactivar/Reactivar success, "No, cancelar" on the delete confirmation,
// a blocked delete's guard message).
func (a *CatalogAdminAdapter) recordDetailResponse() InteractionResponse {
	def, _ := a.registry.Kind(a.activeKind)
	title := catalogRecordDisplayLabel(def, a.activeRecord)
	if !a.activeRecord.Active {
		title += " (Inactivo)"
	}
	return InteractionResponse{
		Messages: []InteractionMessage{StructuredResult{Title: title, Fields: catalogRecordFields(def, a.activeRecord)}},
		Pending:  catalogRecordActionsRequest(def, a.activeRecord),
	}
}

// catalogRecordActionsRequest builds the "¿Qué querés hacer?" menu offered
// from a record's detail view. Desactivar/Reactivar is offered ONLY when
// def.SoftDelete is true (catalog_kind.go) — Service.Deactivate/Reactivate
// would just fail with domain.ErrSoftDeleteUnsupported for the 6 kinds that
// have no Go Active field yet, so the UI never exposes an action guaranteed
// to fail (Generic Descriptor-Driven Engine: the descriptor itself decides,
// no per-kind branch here).
func catalogRecordActionsRequest(def domain.CatalogKind, rec domain.CatalogRecord) ActionRequest {
	actions := []Action{
		{ID: catalogRecordEditActionID, Label: "Editar", Value: catalogRecordEditActionID, Target: ActionTargetAgent},
	}
	if def.SoftDelete {
		if rec.Active {
			actions = append(actions, Action{ID: catalogRecordDeactivateActionID, Label: "Desactivar", Value: catalogRecordDeactivateActionID, Target: ActionTargetAgent})
		} else {
			actions = append(actions, Action{ID: catalogRecordReactivateActionID, Label: "Reactivar", Value: catalogRecordReactivateActionID, Target: ActionTargetAgent})
		}
	}
	actions = append(actions,
		Action{ID: catalogRecordDeleteActionID, Label: "Eliminar", Value: catalogRecordDeleteActionID, Target: ActionTargetAgent},
		Action{ID: catalogRecordBackActionID, Label: "Volver al listado", Value: catalogRecordBackActionID, Target: ActionTargetAgent},
	)
	return ActionRequest{
		ID: catalogRecordActionsKey, Key: catalogRecordActionsKey,
		Question: "¿Qué querés hacer?", Actions: actions,
	}
}

// answerSetActive handles the Desactivar (active=false) / Reactivar
// (active=true) actions (task 7.1): calls the matching narrow interface,
// updates a.activeRecord.Active on success so the refreshed detail view
// (recordDetailResponse) immediately reflects it, and maps any error to a
// Spanish message via catalogLifecycleErrorMessage without leaking a raw Go
// error.
func (a *CatalogAdminAdapter) answerSetActive(ctx context.Context, active bool) (InteractionResponse, error) {
	def, _ := a.registry.Kind(a.activeKind)
	verb := "desactivar"
	var err error
	if active {
		verb = "reactivar"
		err = a.reactivator.Reactivate(ctx, a.activeKind, a.activeRecord.ID)
	} else {
		err = a.deactivator.Deactivate(ctx, a.activeKind, a.activeRecord.ID)
	}
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: catalogLifecycleErrorMessage(def, verb, err)},
		}}, nil
	}
	a.activeRecord.Active = active
	return a.recordDetailResponse(), nil
}

// startDeleteConfirmation begins the "Eliminar" action (task 7.1): a record
// blocked by ≥1 blocking dependency (another catalog-structure kind still
// referencing it) or by a real resource reference is rejected outright, with
// a Spanish message naming the reason and Desactivar offered as the
// alternative (guardedDeleteMessage) — nothing is deleted, and the SAME
// action menu is re-offered so the operator can pick Desactivar right away.
// An unblocked record instead shows a real delete ConfirmationRequest,
// mirroring resources_workspace_dispatch.go's startDeleteConfirmation
// precedent — nothing is deleted at this point either.
func (a *CatalogAdminAdapter) startDeleteConfirmation(ctx context.Context) (InteractionResponse, error) {
	def, _ := a.registry.Kind(a.activeKind)
	message, blocked, err := a.guardedDeleteMessage(ctx, def)
	if err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: "No pude verificar las dependencias. Probá de nuevo en un momento."},
		}}, nil
	}
	if blocked {
		response := a.recordDetailResponse()
		response.Messages = append([]InteractionMessage{ErrorMessage{Text: message}}, response.Messages...)
		return response, nil
	}
	title := catalogRecordDisplayLabel(def, a.activeRecord)
	question := fmt.Sprintf("Eliminar %s\n\n%s\n\n¿Seguro que querés eliminar este registro?", strings.ToLower(def.Singular), title)
	return InteractionResponse{Pending: ConfirmationRequest{
		ID: catalogDeleteConfirmKey, Key: catalogDeleteConfirmKey, Question: question,
		ConfirmLabel: "Sí, eliminar", CancelLabel: "No, cancelar",
	}}, nil
}

// guardedDeleteMessage is task 7.1's dependency-guard check, generic across
// every registered kind (spec: Generic Descriptor-Driven Engine — not
// special-cased to Opciones even though that is the task's own named
// example). It checks two independent signals: (1) dependencyChecker.
// Dependencies — other catalog-structure records still referencing this one
// (design §6's RelationDescriptor.Blocking probes), which yields an exact
// count ("N Tipos"); (2) referenceChecker.ReferencedByResources — real
// Resource instances, which yields only a boolean (the repository probe has
// no count for this), so that message is phrased without a specific number.
// The first blocking signal found wins; a record with neither is unblocked.
func (a *CatalogAdminAdapter) guardedDeleteMessage(ctx context.Context, def domain.CatalogKind) (message string, blocked bool, err error) {
	deps, err := a.dependencyChecker.Dependencies(ctx, a.activeKind, a.activeRecord.ID)
	if err != nil {
		return "", false, err
	}
	for _, dep := range deps {
		if dep.Blocking && dep.Count > 0 {
			depDef, _ := a.registry.Kind(dep.Kind)
			return fmt.Sprintf("El registro de %s está siendo utilizado por %d %s y no puede eliminarse. Podés desactivarlo en su lugar.",
				def.Singular, dep.Count, strings.ToLower(depDef.Plural)), true, nil
		}
	}
	referenced, err := a.referenceChecker.ReferencedByResources(ctx, a.activeKind, a.activeRecord.ID)
	if err != nil {
		return "", false, err
	}
	if referenced {
		return fmt.Sprintf("El registro de %s está siendo utilizado por recursos existentes y no puede eliminarse. Podés desactivarlo en su lugar.",
			def.Singular), true, nil
	}
	return "", false, nil
}

// answerDeleteConfirmation handles catalogDeleteConfirmKey's answer: "no"
// returns to the SAME detail view (mirrors
// resources_workspace_dispatch.go's answerDeleteConfirmation precedent);
// "yes" calls the deleter and confirms what happened, or maps the error to a
// Spanish message without leaking a raw Go error.
func (a *CatalogAdminAdapter) answerDeleteConfirmation(ctx context.Context, value string) (InteractionResponse, error) {
	if value != "yes" {
		return a.recordDetailResponse(), nil
	}
	def, _ := a.registry.Kind(a.activeKind)
	title := catalogRecordDisplayLabel(def, a.activeRecord)
	if err := a.deleter.Delete(ctx, a.activeKind, a.activeRecord.ID); err != nil {
		return InteractionResponse{Messages: []InteractionMessage{
			ErrorMessage{Text: catalogLifecycleErrorMessage(def, "eliminar", err)},
		}}, nil
	}
	return InteractionResponse{Messages: []InteractionMessage{
		TextMessage{Text: fmt.Sprintf("Se eliminó el registro de %s: %s", def.Singular, title)},
	}}, nil
}
