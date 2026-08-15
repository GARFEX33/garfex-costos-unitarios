package tui

import (
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// commandsTestClasses is a raw (unsorted, mixed active/inactive) class slice
// used by every test in this file to exercise the pure commands.go builders
// (recursos-maestro design §7) in isolation, without a real
// domain.ResourceCatalog.
func commandsTestClasses() []domain.ResourceClass {
	return []domain.ResourceClass{
		{Code: "B_CLASS", Name: "Beta", Plural: "Betas", Slug: "betas", Order: 2, Active: true},
		{Code: "A_CLASS", Name: "Alpha", Plural: "Alphas", Slug: "alphas", Order: 1, Active: true},
		{Code: "C_CLASS", Name: "Charlie Later", Plural: "Charlies", Slug: "charlies", Order: 2, Active: true},
		{Code: "INACTIVE", Name: "Inactivo", Plural: "Inactivos", Slug: "inactivos", Order: 0, Active: false},
	}
}

// TestSortedActiveClassesOrdersByOrderThenName proves sortedActiveClasses
// sorts deterministically by Order, then Name for ties.
func TestSortedActiveClassesOrdersByOrderThenName(t *testing.T) {
	sorted := sortedActiveClasses(commandsTestClasses())
	if len(sorted) != 3 {
		t.Fatalf("len(sorted) = %d, want 3 (the inactive class excluded)", len(sorted))
	}
	got := make([]string, len(sorted))
	for i, class := range sorted {
		got[i] = class.Code
	}
	want := []string{"A_CLASS", "B_CLASS", "C_CLASS"} // Order 1 first; Order-2 tie broken by Name (Beta < Charlie Later)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortedActiveClasses order = %v, want %v", got, want)
		}
	}
}

// TestSortedActiveClassesExcludesInactive proves an Active:false class never
// appears in sortedActiveClasses' output.
func TestSortedActiveClassesExcludesInactive(t *testing.T) {
	for _, class := range sortedActiveClasses(commandsTestClasses()) {
		if class.Code == "INACTIVE" {
			t.Fatalf("sortedActiveClasses = includes %q, want Active:false excluded", class.Code)
		}
	}
}

// TestBuildWorkspaceDescriptorsExcludesInactiveClasses proves that running
// the real pipeline (sortedActiveClasses -> buildWorkspaceDescriptors) never
// produces a workspace for an inactive class, and always includes exactly
// one extra descriptor for the unfiltered "/recursos" workspace.
func TestBuildWorkspaceDescriptorsExcludesInactiveClasses(t *testing.T) {
	classes := sortedActiveClasses(commandsTestClasses())
	descriptors := buildWorkspaceDescriptors(classes, func(classCode string) InteractionAgent { return &fakeCatalogAgent{} })
	if len(descriptors) != len(classes)+1 {
		t.Fatalf("len(descriptors) = %d, want %d (one per active class plus the unfiltered workspace)", len(descriptors), len(classes)+1)
	}
	for _, d := range descriptors {
		if d.Slug == "inactivos" {
			t.Fatalf("buildWorkspaceDescriptors = %#v, must not include an inactive class's workspace", descriptors)
		}
	}
}

// TestBuildWorkspaceDescriptorsUnfilteredEntryFirst proves the unfiltered
// "/recursos" descriptor is always first and carries an empty ClassCode.
func TestBuildWorkspaceDescriptorsUnfilteredEntryFirst(t *testing.T) {
	classes := sortedActiveClasses(commandsTestClasses())
	descriptors := buildWorkspaceDescriptors(classes, func(classCode string) InteractionAgent { return &fakeCatalogAgent{} })
	if len(descriptors) == 0 {
		t.Fatal("buildWorkspaceDescriptors returned no descriptors")
	}
	first := descriptors[0]
	if first.Slug != recursosSlug || first.ClassCode != "" {
		t.Fatalf("descriptors[0] = %#v, want the unfiltered recursos workspace first", first)
	}
	for i, class := range classes {
		got := descriptors[i+1]
		if got.Slug != class.Slug || got.ClassCode != class.Code {
			t.Fatalf("descriptors[%d] = %#v, want Slug=%q ClassCode=%q", i+1, got, class.Slug, class.Code)
		}
	}
}

// TestBuildAssistantActionsOrdersStaticStubsAfterClassSubtree proves
// buildAssistantActions puts the class-derived "Recursos" subtree first and
// the untouched out-of-scope stubs (concepts/apu/suppliers) after it, in
// their existing order.
func TestBuildAssistantActionsOrdersStaticStubsAfterClassSubtree(t *testing.T) {
	classes := sortedActiveClasses(commandsTestClasses())
	actions := buildAssistantActions(classes, domain.NewCatalogRegistry().Kinds())
	if len(actions) != 2+len(staticAssistantActions) {
		t.Fatalf("len(actions) = %d, want %d (one Recursos subtree, one Configuración leaf, plus the static stubs)", len(actions), 2+len(staticAssistantActions))
	}
	if actions[0].id != recursosSlug {
		t.Fatalf("actions[0].id = %q, want %q (the class-derived subtree must come first)", actions[0].id, recursosSlug)
	}
	if actions[1].id != configuracionSlug || actions[1].label != "Configuración" {
		t.Fatalf("actions[1] = %#v, want the Configuración leaf right after the Recursos subtree", actions[1])
	}
	for i, stub := range staticAssistantActions {
		if actions[i+2].id != stub.id || actions[i+2].label != stub.label {
			t.Fatalf("actions[%d] = %#v, want the static stub %#v (order preserved, after Recursos+Configuración)", i+2, actions[i+2], stub)
		}
	}
}

// TestBuildCatalogAdminActionsGroupsKindsIntoBusinessAskSubtree proves
// buildCatalogAdminActions groups every registered CatalogKind into the
// original business-ask's proposed menu shape: one root "Catálogo de
// recursos" node with Estructura/Características/Unidades/"Configuración de
// tipos" children, each carrying the registry's own Spanish labels.
func TestBuildCatalogAdminActionsGroupsKindsIntoBusinessAskSubtree(t *testing.T) {
	root := buildCatalogAdminActions(domain.NewCatalogRegistry().Kinds())
	if root.label != "Catálogo de recursos" {
		t.Fatalf("root.label = %q, want %q", root.label, "Catálogo de recursos")
	}
	if len(root.children) != 4 {
		t.Fatalf("len(root.children) = %d, want 4 (Estructura/Características/Unidades/Configuración de tipos)", len(root.children))
	}
	wantLabels := []string{"Estructura", "Características", "Unidades", "Configuración de tipos"}
	for i, want := range wantLabels {
		if root.children[i].label != want {
			t.Fatalf("root.children[%d].label = %q, want %q", i, root.children[i].label, want)
		}
	}
	estructura := root.children[0]
	wantEstructura := []string{"Clases", "Familias", "Tipos"}
	if len(estructura.children) != len(wantEstructura) {
		t.Fatalf("len(estructura.children) = %d, want %d", len(estructura.children), len(wantEstructura))
	}
	for i, want := range wantEstructura {
		if estructura.children[i].label != want {
			t.Fatalf("estructura.children[%d].label = %q, want %q", i, estructura.children[i].label, want)
		}
	}
}

// TestBuildCatalogAdminActionsUnregisteredKindNotExposed proves only
// registered CatalogKinds are exposed — passing an empty kind slice produces
// leaf labels that are all empty strings, never a fabricated CatalogKindCode
// leaking through as a label (spec: "Unregistered tables are not exposed").
func TestBuildCatalogAdminActionsUnregisteredKindNotExposed(t *testing.T) {
	root := buildCatalogAdminActions(nil)
	for _, group := range root.children {
		for _, leaf := range group.children {
			if leaf.id == catalogIdentityActionID {
				continue // the one TUI-only routing sentinel, not backed by a CatalogKind
			}
			if leaf.label != "" {
				t.Fatalf("leaf %q label = %q, want empty when no CatalogKind is registered for it", leaf.id, leaf.label)
			}
		}
	}
}

// TestBuildCatalogAdminWorkspaceSetsSlugAndPaletteActions proves
// buildCatalogAdminWorkspace wires the "Configuración" slug, the given
// agent, and a single-root PaletteActions tree.
func TestBuildCatalogAdminWorkspaceSetsSlugAndPaletteActions(t *testing.T) {
	agent := &fakeCatalogAgent{}
	descriptor := buildCatalogAdminWorkspace(domain.NewCatalogRegistry().Kinds(), agent)
	if descriptor.Slug != configuracionSlug {
		t.Fatalf("descriptor.Slug = %q, want %q", descriptor.Slug, configuracionSlug)
	}
	if descriptor.Agent != InteractionAgent(agent) {
		t.Fatalf("descriptor.Agent = %#v, want the given agent", descriptor.Agent)
	}
	if len(descriptor.PaletteActions) != 1 || descriptor.PaletteActions[0].label != "Catálogo de recursos" {
		t.Fatalf("descriptor.PaletteActions = %#v, want the single Catálogo de recursos root", descriptor.PaletteActions)
	}
}

// TestWorkspaceActionsFallsBackWhenPaletteActionsNil is the explicit
// nil-safety proof task 6.2 calls for: every EXISTING WorkspaceDescriptor
// construction site (which never sets PaletteActions) must still work
// exactly as before.
func TestWorkspaceActionsFallsBackWhenPaletteActionsNil(t *testing.T) {
	descriptor := WorkspaceDescriptor{Slug: "alphas", CreateLabel: "Crear alpha"}
	actions := workspaceActions(descriptor)
	if len(actions) != 1 || actions[0].label != "Crear alpha" || actions[0].id != createResourceActionID {
		t.Fatalf("workspaceActions(%#v) = %#v, want the original single Crear leaf when PaletteActions is nil", descriptor, actions)
	}
}

// TestWorkspaceActionsUsesPaletteActionsWhenSet proves a non-nil
// PaletteActions (the "Configuración" workspace) is returned as-is instead
// of the CreateLabel fallback.
func TestWorkspaceActionsUsesPaletteActionsWhenSet(t *testing.T) {
	custom := []assistantAction{{id: "custom-root", label: "Personalizado"}}
	descriptor := WorkspaceDescriptor{Slug: configuracionSlug, PaletteActions: custom}
	actions := workspaceActions(descriptor)
	if len(actions) != 1 || actions[0].id != "custom-root" {
		t.Fatalf("workspaceActions(%#v) = %#v, want the custom PaletteActions returned unchanged", descriptor, actions)
	}
}

// TestBuildResourceActionsFirstChildIsUnfilteredEntry proves the "Recursos"
// subtree's first child is the explicit "Todos los recursos" leaf (design
// R5), followed by one child per active class in the given order.
func TestBuildResourceActionsFirstChildIsUnfilteredEntry(t *testing.T) {
	classes := sortedActiveClasses(commandsTestClasses())
	tree := buildResourceActions(classes)
	if len(tree.children) != len(classes)+1 {
		t.Fatalf("len(tree.children) = %d, want %d", len(tree.children), len(classes)+1)
	}
	if tree.children[0].id != recursosSlug || tree.children[0].label != "Todos los recursos" {
		t.Fatalf("tree.children[0] = %#v, want the explicit unfiltered \"Todos los recursos\" leaf", tree.children[0])
	}
	for i, class := range classes {
		child := tree.children[i+1]
		if child.id != class.Slug || child.label != class.Plural {
			t.Fatalf("tree.children[%d] = %#v, want id=%q label=%q", i+1, child, class.Slug, class.Plural)
		}
	}
}

// TestBuildResourceActionsExcludesInactiveClass proves an inactive class
// never appears as a child of the "Recursos" palette subtree.
func TestBuildResourceActionsExcludesInactiveClass(t *testing.T) {
	classes := sortedActiveClasses(commandsTestClasses())
	tree := buildResourceActions(classes)
	for _, child := range tree.children {
		if child.id == "inactivos" {
			t.Fatalf("buildResourceActions children = %#v, must exclude an inactive class", tree.children)
		}
	}
}

// TestBuildResourceActionsCarriesAliasesAndKeywords proves each class's
// Aliases/Keywords survive into its palette leaf, so the flat-query matcher
// (see TestPaletteAliasAndKeywordMatching) has something real to match.
func TestBuildResourceActionsCarriesAliasesAndKeywords(t *testing.T) {
	classes := []domain.ResourceClass{
		{Code: "A_CLASS", Name: "Alpha", Plural: "Alphas", Slug: "alphas", Order: 1, Active: true, Aliases: []string{"alfa"}, Keywords: []string{"primera letra"}},
	}
	tree := buildResourceActions(classes)
	if len(tree.children) != 2 {
		t.Fatalf("len(tree.children) = %d, want 2", len(tree.children))
	}
	leaf := tree.children[1]
	if len(leaf.aliases) != 1 || leaf.aliases[0] != "alfa" {
		t.Fatalf("leaf.aliases = %v, want [alfa]", leaf.aliases)
	}
	if len(leaf.keywords) != 1 || leaf.keywords[0] != "primera letra" {
		t.Fatalf("leaf.keywords = %v, want [primera letra]", leaf.keywords)
	}
}

// TestWorkspaceActionsUsesDescriptorCreateLabel proves workspaceActions
// builds a single "Crear ..." leaf from the descriptor's own CreateLabel —
// the per-descriptor replacement for the old shared materialsActions var.
func TestWorkspaceActionsUsesDescriptorCreateLabel(t *testing.T) {
	descriptor := WorkspaceDescriptor{Slug: "alphas", Title: "GARFEX / ALPHAS", CreateLabel: "Crear alpha"}
	actions := workspaceActions(descriptor)
	if len(actions) != 1 || actions[0].label != "Crear alpha" || actions[0].id != createResourceActionID {
		t.Fatalf("workspaceActions(%#v) = %#v, want exactly one %q leaf labeled %q", descriptor, actions, createResourceActionID, "Crear alpha")
	}
}

// TestPaletteAliasAndKeywordMatching proves the "/" palette's flat-query
// matcher (filterOptions over actionOptions) matches an action's Aliases and
// Keywords, not just its Label (design R6) — using a synthetic action so
// this test does not depend on the real catalog's own alias/keyword data.
func TestPaletteAliasAndKeywordMatching(t *testing.T) {
	actions := []assistantAction{
		{id: "widgets", label: "Widgets", aliases: []string{"gadget"}, keywords: []string{"herramienta especial"}},
	}
	options := actionOptions(actions)
	for _, query := range []string{"widg", "gadget", "herramienta", "especial"} {
		filtered := filterOptions(options, query)
		if len(filtered) != 1 || filtered[0].ID != "widgets" {
			t.Fatalf("filterOptions(query=%q) = %#v, want exactly the Widgets action (matched via label/alias/keyword)", query, filtered)
		}
	}
	if filtered := filterOptions(options, "nomatch"); len(filtered) != 0 {
		t.Fatalf("filterOptions(query=%q) = %#v, want no matches", "nomatch", filtered)
	}
}
