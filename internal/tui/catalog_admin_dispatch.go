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
	lister   catalogLister
	getter   catalogGetter
	creator  catalogRecordCreator
	updater  catalogRecordUpdater
	registry domain.CatalogRegistry
	// activeKind is the kind whose "existing record or crear nueva/o" menu
	// (startKindMenu) is currently pending, "" when none is.
	activeKind domain.CatalogKindCode
	// editor holds the in-progress create/edit flow, if any; nil means none
	// is in progress. See catalog_admin.go.
	editor *catalogEditorState
}

// NewCatalogAdminAdapter returns a CatalogAdminAdapter backed by lister/
// getter/creator/updater (satisfied structurally by *catalogo.Service in
// production — composed once in cmd/garfex/main.go, see the recursos
// analogue there) and registry (the fixed set of administrable CatalogKinds,
// design §3).
func NewCatalogAdminAdapter(lister catalogLister, getter catalogGetter, creator catalogRecordCreator, updater catalogRecordUpdater, registry domain.CatalogRegistry) *CatalogAdminAdapter {
	return &CatalogAdminAdapter{lister: lister, getter: getter, creator: creator, updater: updater, registry: registry}
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
		return a.openRecordForEdit(ctx, a.activeKind, input.Value)
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
