package tui

import (
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// TestBuildWorkspaceDescriptorsWrapsCatalogActiveClasses proves the exported
// BuildWorkspaceDescriptors (recursos-maestro design §7/§8 — PR9's
// resolution of PR8's flagged export/naming gap) turns a real
// domain.ResourceCatalog's active classes into the same shape
// buildWorkspaceDescriptors already produces from a raw class slice: the
// unfiltered "/recursos" entry first, then one per active class in
// ActiveClasses' own Order-then-Name order.
func TestBuildWorkspaceDescriptorsWrapsCatalogActiveClasses(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	agentFor := func(string) InteractionAgent { return &fakeCatalogAgent{} }
	descriptors := BuildWorkspaceDescriptors(catalog, agentFor)

	active := catalog.ActiveClasses()
	if want := len(active) + 1; len(descriptors) != want {
		t.Fatalf("len(descriptors) = %d, want %d (unfiltered + %d active classes)", len(descriptors), want, len(active))
	}
	if descriptors[0].Slug != recursosSlug || descriptors[0].ClassCode != "" {
		t.Fatalf("descriptors[0] = %#v, want the unfiltered /recursos entry first", descriptors[0])
	}
	for i, class := range active {
		got := descriptors[i+1]
		if got.Slug != class.Slug || got.ClassCode != class.Code {
			t.Fatalf("descriptors[%d] = %#v, want Slug=%q ClassCode=%q", i+1, got, class.Slug, class.Code)
		}
	}
}

// TestNewWithCatalogGlobalPaletteIsCatalogDriven closes PR8's second flagged
// gap: the real running app's global (non-workspace) "/" palette must be
// catalog-driven, not the flat legacy literal. Real domain.SeedResourceCatalog()
// seed data drives this end to end through NewWithCatalog, the constructor
// cmd/garfex/main.go uses.
func TestNewWithCatalogGlobalPaletteIsCatalogDriven(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	agentFor := func(string) InteractionAgent { return &fakeCatalogAgent{} }
	m := NewWithCatalog(Handlers{}, NewFakeAgent(), catalog, agentFor, domain.NewCatalogRegistry(), &fakeCatalogAgent{})

	m, _ = update(t, m, key('/'))
	options := filterOptions(actionOptions(m.paletteActions), m.paletteQuery)
	topLabels := make([]string, len(options))
	for i, o := range options {
		topLabels[i] = o.Label
	}
	if containsString(topLabels, "Materiales Maestros") {
		t.Fatalf("catalog-driven palette = %v, must not show the flat legacy label", topLabels)
	}
	if !containsString(topLabels, "Recursos") {
		t.Fatalf("catalog-driven palette = %v, want the catalog-derived Recursos subtree", topLabels)
	}
	for _, want := range []string{"Conceptos", "APU", "Proveedores"} {
		if !containsString(topLabels, want) {
			t.Fatalf("catalog-driven palette = %v, want the static stub %q preserved", topLabels, want)
		}
	}
}

// TestNewWithCatalogClassWorkspaceReachesItsOwnAgent proves a catalog-derived
// class workspace (built via agentFor injection) is reachable through the
// registry exactly like any other registered workspace, closing the loop
// from real catalog data to a real per-class InteractionAgent.
func TestNewWithCatalogClassWorkspaceReachesItsOwnAgent(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	materials := &fakeCatalogAgent{}
	agentFor := func(classCode string) InteractionAgent {
		if classCode == "MATERIAL" {
			return materials
		}
		return &fakeCatalogAgent{}
	}
	m := NewWithCatalog(Handlers{}, NewFakeAgent(), catalog, agentFor, domain.NewCatalogRegistry(), &fakeCatalogAgent{})

	if ok := m.enterWorkspace("materiales"); !ok {
		t.Fatal(`enterWorkspace("materiales") = false, want true`)
	}
	m = submitText(t, m, "hola")
	if materials.calls != 1 {
		t.Fatalf("materials agent calls = %d, want 1 (the real class-scoped agent must have been reached)", materials.calls)
	}
}

// TestNewWithCatalogUnfilteredWorkspaceUsesEmptyClassCode proves the
// unfiltered "/recursos" workspace calls agentFor with an empty classCode,
// matching BuildWorkspaceDescriptors' own contract.
func TestNewWithCatalogUnfilteredWorkspaceUsesEmptyClassCode(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	var gotClassCodes []string
	agentFor := func(classCode string) InteractionAgent {
		gotClassCodes = append(gotClassCodes, classCode)
		return &fakeCatalogAgent{}
	}
	NewWithCatalog(Handlers{}, NewFakeAgent(), catalog, agentFor, domain.NewCatalogRegistry(), &fakeCatalogAgent{})

	if len(gotClassCodes) == 0 || gotClassCodes[0] != "" {
		t.Fatalf("agentFor call order = %v, want the unfiltered entry (empty classCode) called first", gotClassCodes)
	}
}

// TestNewWithCatalogRegistersConfigurationWorkspace proves the
// "Configuración" workspace (design D13) is reachable through the registry
// exactly like any other registered workspace, wired to the given
// catalogAgent.
func TestNewWithCatalogRegistersConfigurationWorkspace(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	agentFor := func(string) InteractionAgent { return &fakeCatalogAgent{} }
	catalogAgent := &fakeCatalogAgent{}
	m := NewWithCatalog(Handlers{}, NewFakeAgent(), catalog, agentFor, domain.NewCatalogRegistry(), catalogAgent)

	if ok := m.enterWorkspace(configuracionSlug); !ok {
		t.Fatal(`enterWorkspace("configuracion") = false, want true`)
	}
	m = submitText(t, m, "hola")
	if catalogAgent.calls != 1 {
		t.Fatalf("catalog admin agent calls = %d, want 1 (the real Configuración agent must have been reached)", catalogAgent.calls)
	}
}
