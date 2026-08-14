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
	actions := buildAssistantActions(classes)
	if len(actions) != 1+len(staticAssistantActions) {
		t.Fatalf("len(actions) = %d, want %d (one Recursos subtree plus the static stubs)", len(actions), 1+len(staticAssistantActions))
	}
	if actions[0].id != recursosSlug {
		t.Fatalf("actions[0].id = %q, want %q (the class-derived subtree must come first)", actions[0].id, recursosSlug)
	}
	for i, stub := range staticAssistantActions {
		if actions[i+1].id != stub.id || actions[i+1].label != stub.label {
			t.Fatalf("actions[%d] = %#v, want the static stub %#v (order preserved, after the class subtree)", i+1, actions[i+1], stub)
		}
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
