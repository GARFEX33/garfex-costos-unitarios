package tui

import (
	"sort"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// recursosSlug is both the unfiltered "/recursos" workspace's registry slug
// (WorkspaceDescriptor.Slug) and the "Recursos" palette subtree's own node
// id (recursos-maestro design §7).
const recursosSlug = "recursos"

// configuracionSlug is both the "Configuración" workspace's registry slug
// (WorkspaceDescriptor.Slug) and its top-level Assistant palette leaf id
// (design D13/§8).
const configuracionSlug = "configuracion"

const searchResourcesActionID = "search-resources"

type assistantAction struct {
	id       string
	label    string
	aliases  []string
	keywords []string
	children []assistantAction
}

// assistantActions is the Assistant's own "/" palette tree (top-level
// screen, outside any registered workspace). PR8 (recursos-maestro design
// §7) adds the class-driven builders below (sortedActiveClasses,
// buildResourceActions, buildWorkspaceDescriptors, buildAssistantActions,
// workspaceActions) as pure, independently testable functions; wiring this
// package-level var itself to a real domain.ResourceCatalog — so the
// running app's global palette becomes catalog-driven, not just testable in
// isolation — is PR9's job (cmd/garfex/main.go, design §8), the same PR that
// constructs the real catalog in the first place.
var assistantActions = []assistantAction{
	{id: "materials", label: "Materiales Maestros"},
}

// sortedActiveClasses filters classes down to the Active ones and sorts them
// deterministically by Order, then Name for ties (recursos-maestro design
// §7) — the same rule domain.ResourceCatalog.ActiveClasses() already applies
// to a whole catalog, exposed here as an independently testable pure
// function over a raw class slice (e.g. catalog.Classes).
func sortedActiveClasses(classes []domain.ResourceClass) []domain.ResourceClass {
	active := make([]domain.ResourceClass, 0, len(classes))
	for _, class := range classes {
		if class.Active {
			active = append(active, class)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].Order != active[j].Order {
			return active[i].Order < active[j].Order
		}
		return active[i].Name < active[j].Name
	})
	return active
}

// buildResourceActions builds the "/" palette's "Recursos" subtree: an
// explicit "Todos los recursos" first child (design R5 — model.go's palette
// confirm handler descends into any node with children before ever checking
// whether that same node is also a workspace-launchable leaf, so the
// unfiltered "/recursos" workspace needs its own dedicated leaf rather than
// reusing the parent node's id), followed by one child per active class,
// each carrying its class's own Aliases/Keywords for the flat-query matcher
// (design R6, see filterOptions).
func buildResourceActions(classes []domain.ResourceClass) assistantAction {
	children := make([]assistantAction, 0, len(classes)+1)
	children = append(children, assistantAction{id: recursosSlug, label: "Todos los recursos"})
	for _, class := range classes {
		children = append(children, assistantAction{
			id: class.Slug, label: class.Plural,
			aliases: class.Aliases, keywords: class.Keywords,
		})
	}
	return assistantAction{id: recursosSlug, label: "Recursos", children: children}
}

// BuildWorkspaceDescriptors is the exported entry point cmd/garfex/main.go
// uses (recursos-maestro design §8) — it resolves PR8's flagged
// exported-ness/naming gap between design §7 (unexported builders) and
// design §8 (an exported, catalog-level call from a different package) by
// wrapping buildWorkspaceDescriptors with catalog.ActiveClasses(), so a
// caller outside package tui never needs to know about the raw
// []domain.ResourceClass shape or sortedActiveClasses.
func BuildWorkspaceDescriptors(catalog domain.ResourceCatalog, agentFor func(classCode string) InteractionAgent) []WorkspaceDescriptor {
	return buildWorkspaceDescriptors(catalog.ActiveClasses(), agentFor)
}

// buildWorkspaceDescriptors turns an active-class list into the registry
// descriptors NewWithWorkspaces needs: the unfiltered "/recursos" workspace
// first (ClassCode ""), then one class-scoped workspace per active class.
// agentFor is injected (recursos-maestro design §5/§8) so this stays a pure
// function over data — the exact structural hook that lets a synthetic
// catalog (a class added purely as data, see synthetic_class_test.go) drive
// this same code path in a test without any per-class branch.
func buildWorkspaceDescriptors(classes []domain.ResourceClass, agentFor func(classCode string) InteractionAgent) []WorkspaceDescriptor {
	descriptors := make([]WorkspaceDescriptor, 0, len(classes)+1)
	descriptors = append(descriptors, WorkspaceDescriptor{
		Slug:          recursosSlug,
		Title:         "GARFEX / RECURSOS",
		CreateLabel:   "Crear recurso",
		SearchOnEnter: true,
		Agent:         agentFor(""),
	})
	for _, class := range classes {
		descriptors = append(descriptors, WorkspaceDescriptor{
			Slug:          class.Slug,
			Title:         "GARFEX / " + strings.ToUpper(class.Plural),
			CreateLabel:   "Crear " + strings.ToLower(class.Name),
			SearchOnEnter: true,
			ClassCode:     class.Code,
			Agent:         agentFor(class.Code),
		})
	}
	return descriptors
}

// buildAssistantActions exposes only capabilities backed by a working
// workspace. Future modules stay hidden until their end-to-end flow exists.
func buildAssistantActions(classes []domain.ResourceClass, kinds []domain.CatalogKind) []assistantAction {
	actions := make([]assistantAction, 0, 2)
	actions = append(actions, buildResourceActions(classes))
	actions = append(actions, assistantAction{id: configuracionSlug, label: "Configuración"})
	return actions
}

// catalogKindLeaf builds one kind's palette leaf action — id routes through
// catalogOpenActionID (CatalogAdminAdapter.Respond strips the prefix and
// looks the kind up in its own registry), label is always the registry's own
// Spanish Plural, never a raw CatalogKindCode (spec: Spanish-Only
// Catalog-Admin UI).
func catalogKindLeaf(byCode map[domain.CatalogKindCode]domain.CatalogKind, code domain.CatalogKindCode) assistantAction {
	return assistantAction{id: catalogOpenActionID(code), label: byCode[code].Plural}
}

// buildCatalogAdminActions builds the catalog action hierarchy consumed by
// the "Configuración" workspace. The workspace exposes its children directly;
// the compatibility root only groups every registered CatalogKind into the
// original business-ask's proposed shape (Estructura /
// Características / Unidades / Configuración de tipos) — group/leaf labels
// come from the registry's own Spanish Singular/Plural wherever a real kind
// backs the leaf, never hardcoded per-kind strings, so this stays
// descriptor-driven (spec: Generic Descriptor-Driven Engine; "Unregistered
// tables are not exposed" — every leaf here resolves through a real
// registered CatalogKindCode; "Identidad" is the sole exception, a
// TUI-layer-only routing sentinel over the same KindAttributeBinding flow as
// "Aplicabilidad", since Identidad is not its own CatalogKind, design
// reconciliation). KindUnitPolicy and KindOptionRelation are not named in
// the business ask's literal proposed shape but are still registered,
// administrable CatalogKinds (design §3) — placed under their closest
// natural grouping (Unidades, Características) so every registered kind
// stays reachable, per the registry-driven completeness the spec's
// "Generic Descriptor-Driven Engine" requirement implies.
func buildCatalogAdminActions(kinds []domain.CatalogKind) assistantAction {
	byCode := make(map[domain.CatalogKindCode]domain.CatalogKind, len(kinds))
	for _, k := range kinds {
		byCode[k.Code] = k
	}
	estructura := assistantAction{id: "catalog-menu-estructura", label: "Estructura", children: []assistantAction{
		catalogKindLeaf(byCode, domain.KindClass),
		catalogKindLeaf(byCode, domain.KindFamily),
		catalogKindLeaf(byCode, domain.KindType),
	}}
	caracteristicas := assistantAction{id: "catalog-menu-caracteristicas", label: "Características", children: []assistantAction{
		catalogKindLeaf(byCode, domain.KindAttributeDefinition),
		catalogKindLeaf(byCode, domain.KindOptionSet),
		catalogKindLeaf(byCode, domain.KindOption),
		catalogKindLeaf(byCode, domain.KindOptionRelation),
	}}
	unidades := assistantAction{id: "catalog-menu-unidades", label: "Unidades", children: []assistantAction{
		catalogKindLeaf(byCode, domain.KindUnit),
		catalogKindLeaf(byCode, domain.KindUnitPolicy),
	}}
	tipoConfig := assistantAction{id: "catalog-menu-config-tipos", label: "Configuración de tipos", children: []assistantAction{
		catalogKindLeaf(byCode, domain.KindAttributeBinding),
		{id: catalogIdentityActionID, label: "Identidad"},
		catalogKindLeaf(byCode, domain.KindPresentationField),
	}}
	// crearEstructura is the guided wizard's own entry point (task 9.1/9.2,
	// catalog_wizard.go) — a leaf, not a group, since it starts a sequence
	// rather than opening a per-kind menu; not backed by any single
	// CatalogKind (like catalogIdentityActionID), so it stays a static label
	// regardless of what kinds are registered.
	crearEstructura := assistantAction{id: catalogWizardActionID, label: "Crear estructura de recursos"}
	return assistantAction{id: "catalog-menu-root", label: "Catálogo de recursos", children: []assistantAction{
		crearEstructura, estructura, caracteristicas, unidades, tipoConfig,
	}}
}

// buildCatalogAdminWorkspace builds the "Configuración" WorkspaceDescriptor
// (design D13) — agent is the production CatalogAdminAdapter (or a fake in
// tests), and PaletteActions exposes buildCatalogAdminActions' useful children
// directly, without rendering its compatibility root.
func buildCatalogAdminWorkspace(kinds []domain.CatalogKind, agent InteractionAgent) WorkspaceDescriptor {
	actions := buildCatalogAdminActions(kinds)
	return WorkspaceDescriptor{
		Slug:           configuracionSlug,
		Title:          "GARFEX / CONFIGURACIÓN",
		Agent:          agent,
		PaletteActions: actions.children,
	}
}

// workspaceActions is the "/" palette's action tree while inside ANY
// registered workspace slot (see Model.activePaletteActions). Design D13:
// PaletteActions == nil falls back to the scoped Crear/Buscar actions built
// from CreateLabel. Resource descriptors use that fallback for the command
// palette while SearchOnEnter independently makes search their direct entry;
// a non-nil PaletteActions (the "Configuración" workspace) is returned as-is.
func workspaceActions(d WorkspaceDescriptor) []assistantAction {
	if d.PaletteActions != nil {
		return d.PaletteActions
	}
	return []assistantAction{
		{id: createResourceActionID, label: d.CreateLabel},
		{id: searchResourcesActionID, label: "Buscar recursos"},
	}
}

func flattenLeafActions(actions []assistantAction) []assistantAction {
	var result []assistantAction
	for _, action := range actions {
		if len(action.children) > 0 {
			result = append(result, flattenLeafActions(action.children)...)
			continue
		}
		result = append(result, action)
	}
	return result
}

func actionOptions(actions []assistantAction) []Option {
	options := make([]Option, 0, len(actions))
	for _, action := range actions {
		var searchTerms []string
		if len(action.aliases) > 0 || len(action.keywords) > 0 {
			searchTerms = make([]string, 0, len(action.aliases)+len(action.keywords))
			searchTerms = append(searchTerms, action.aliases...)
			searchTerms = append(searchTerms, action.keywords...)
		}
		options = append(options, Option{
			ID: action.id, Label: action.label, Value: action.id,
			SearchTerms: searchTerms,
		})
	}
	return options
}

func manualMaterialOptions() []Option {
	return []Option{
		{Label: "THW-LS 10 AWG Negro", Value: "thw-ls-10-black"},
		{Label: "THW-LS 12 AWG Blanco", Value: "thw-ls-12-white"},
		{Label: "XHHW-2 10 AWG Negro", Value: "xhhw-2-10-black"},
	}
}
