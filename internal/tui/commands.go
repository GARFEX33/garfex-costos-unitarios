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
	{id: "concepts", label: "Conceptos"},
	{id: "apu", label: "APU"},
	{id: "suppliers", label: "Proveedores"},
}

// staticAssistantActions are the out-of-scope stubs buildAssistantActions
// appends after the class-derived "Recursos" subtree (recursos-maestro
// design §7) — Conceptos/APU/Proveedores are not resource classes and stay
// exactly as they are today.
var staticAssistantActions = []assistantAction{
	{id: "concepts", label: "Conceptos"},
	{id: "apu", label: "APU"},
	{id: "suppliers", label: "Proveedores"},
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
		Slug:        recursosSlug,
		Title:       "GARFEX / RECURSOS",
		CreateLabel: "Crear recurso",
		Agent:       agentFor(""),
	})
	for _, class := range classes {
		descriptors = append(descriptors, WorkspaceDescriptor{
			Slug:        class.Slug,
			Title:       "GARFEX / " + strings.ToUpper(class.Plural),
			CreateLabel: "Crear " + strings.ToLower(class.Name),
			ClassCode:   class.Code,
			Agent:       agentFor(class.Code),
		})
	}
	return descriptors
}

// buildAssistantActions composes the class-derived "Recursos" subtree with
// the untouched out-of-scope static stubs, class subtree first (design §7).
func buildAssistantActions(classes []domain.ResourceClass) []assistantAction {
	actions := make([]assistantAction, 0, 1+len(staticAssistantActions))
	actions = append(actions, buildResourceActions(classes))
	actions = append(actions, staticAssistantActions...)
	return actions
}

// workspaceActions is the "/" palette's action tree while inside ANY
// registered resources workspace slot (see Model.activePaletteActions) —
// replaces the old single shared materialsActions var with a per-descriptor
// tree, so each workspace's own CreateLabel (not just Materiales') renders
// correctly.
func workspaceActions(d WorkspaceDescriptor) []assistantAction {
	return []assistantAction{{id: createResourceActionID, label: d.CreateLabel}}
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
