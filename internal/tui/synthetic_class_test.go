package tui

import (
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/charmbracelet/x/ansi"
)

// syntheticServiceClassCatalog returns the REAL production catalog
// (domain.SeedResourceCatalog(), the 3-class MATERIAL/MANO_DE_OBRA/
// EQUIPO_HERRAMIENTA seed) with a 4th class — SERVICIO_ESPECIALIZADO —
// appended as PURE DATA: one family, one type, one controlled-option
// attribute with two real option values, and a unit policy reusing the
// existing "PZA" unit. Zero new Go types, zero new files beyond this test,
// zero modified production code specific to this class — this is the literal
// proof for AC #3 ("not functionally limited to three hardcoded classes")
// and AC #15 ("a new Class can be added to the model without creating
// another CRUD"), per recursos-maestro design §5's whole rationale for
// injecting the catalog into ResourcesWorkspaceAdapter instead of resolving
// it via a package-level constructor call.
func syntheticServiceClassCatalog() domain.ResourceCatalog {
	catalog := domain.SeedResourceCatalog()

	catalog.Classes = append(catalog.Classes, domain.ResourceClass{
		Code: "SERVICIO_ESPECIALIZADO", Name: "Servicio especializado", Plural: "Servicios especializados",
		Slug: "servicios-especializados", Order: 4, Active: true,
		Aliases: []string{"servicio", "especializado"}, Keywords: []string{"servicio especializado"},
	})
	catalog.Families = append(catalog.Families, domain.ResourceFamily{
		ClassCode: "SERVICIO_ESPECIALIZADO", Code: "INSTALACION", Name: "Instalación", Active: true,
	})
	catalog.Types = append(catalog.Types, domain.ResourceType{
		ClassCode: "SERVICIO_ESPECIALIZADO", FamilyCode: "INSTALACION", Code: "PUESTA_EN_MARCHA", Name: "Puesta en marcha", Active: true,
	})
	catalog.Definitions = append(catalog.Definitions, domain.AttributeDefinition{
		Code: "nivel_servicio", Name: "Nivel de servicio", ValueType: domain.ValueTypeControlledOption, DefaultIdentityParticipates: true,
	})
	catalog.Attributes = append(catalog.Attributes, domain.ResourceAttribute{
		ClassCode: "SERVICIO_ESPECIALIZADO", FamilyCode: "INSTALACION", TypeCode: "PUESTA_EN_MARCHA",
		Definition: domain.AttributeDefinition{Code: "nivel_servicio", Name: "Nivel de servicio", ValueType: domain.ValueTypeControlledOption, DefaultIdentityParticipates: true},
		Mode:       domain.ModeRequired, IdentityParticipates: true,
	})
	catalog.Options = append(catalog.Options,
		domain.AttributeOption{AttributeCode: "nivel_servicio", Code: "BASICO", Label: "Básico", Active: true},
		domain.AttributeOption{AttributeCode: "nivel_servicio", Code: "AVANZADO", Label: "Avanzado", Active: true},
	)
	catalog.UnitPolicies = append(catalog.UnitPolicies, domain.ResourceUnitPolicy{
		ClassCode: "SERVICIO_ESPECIALIZADO", FamilyCode: "INSTALACION", UnitCode: "PZA", Allowed: true, Suggested: true,
	})

	return catalog
}

// TestSyntheticFourthClassIsStructurallyValid is a cheap upfront guard: the
// synthetic catalog above must itself pass Validate() before it is trusted
// to drive the rest of this test — a defect here would otherwise surface as
// a confusing failure deep inside the TUI flow below instead of at its root
// cause.
func TestSyntheticFourthClassIsStructurallyValid(t *testing.T) {
	if err := syntheticServiceClassCatalog().Validate(); err != nil {
		t.Fatalf("syntheticServiceClassCatalog().Validate() = %v, want nil", err)
	}
}

// TestSyntheticFourthClassAppearsInAssistantActions proves the synthetic
// class flows through buildAssistantActions (design §7) exactly like a real
// seeded class: as a leaf inside the "Recursos" subtree, carrying its own
// Aliases/Keywords.
func TestSyntheticFourthClassAppearsInAssistantActions(t *testing.T) {
	catalog := syntheticServiceClassCatalog()
	actions := buildAssistantActions(catalog.ActiveClasses())
	resourceTree := actions[0]
	var found *assistantAction
	for i := range resourceTree.children {
		if resourceTree.children[i].id == "servicios-especializados" {
			found = &resourceTree.children[i]
		}
	}
	if found == nil {
		t.Fatalf("buildAssistantActions(catalog.ActiveClasses()) Recursos children = %#v, want a %q leaf for the synthetic class", resourceTree.children, "servicios-especializados")
	}
	if found.label != "Servicios especializados" {
		t.Fatalf("synthetic leaf label = %q, want %q", found.label, "Servicios especializados")
	}
	if len(found.aliases) != 2 || found.aliases[0] != "servicio" {
		t.Fatalf("synthetic leaf aliases = %v, want the catalog's own [servicio especializado]", found.aliases)
	}
}

// TestSyntheticFourthClassCreateFlowRendersItsOwnCatalogData is THE proof
// (AC #3/#15): buildAssistantActions -> buildWorkspaceDescriptors ->
// tui.NewWithWorkspaces -> m.enterWorkspace(synthetic slug) -> the real "/"
// palette's "Crear ..." leaf -> the real create-flow state machine
// (startCreateEditor and everything it calls, resource_editor.go) — driven
// with the same synthetic keypress/selection sequence this project's other
// TUI tests already use (see workspace_switch_test.go/model_test.go) — must
// actually render and let the user select the synthetic class's own
// Familia ("Instalación"), Tipo ("Puesta en marcha"), and Atributo ("Nivel
// de servicio", options "Básico"/"Avanzado"), ending in a persisted Resource
// scoped to ClassCode "SERVICIO_ESPECIALIZADO" — not just a reachable menu
// entry. Zero production code was changed to make this pass beyond what
// 8.3's generic, catalog-driven builders already provide for every class.
func TestSyntheticFourthClassCreateFlowRendersItsOwnCatalogData(t *testing.T) {
	catalog := syntheticServiceClassCatalog()
	classes := catalog.ActiveClasses()

	creator := &fakeResourceCreator{}
	agentFor := func(classCode string) InteractionAgent {
		return NewResourcesWorkspaceAdapter(
			&fakeResourceSearcher{}, &fakeResourceGetter{}, &fakeResourceDescriber{},
			creator, &fakeResourceUpdater{}, &fakeResourceDeleter{},
			catalog, classCode,
		)
	}
	descriptors := buildWorkspaceDescriptors(classes, agentFor)

	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), descriptors)

	const syntheticSlug = "servicios-especializados"
	if ok := m.enterWorkspace(syntheticSlug); !ok {
		t.Fatalf("enterWorkspace(%q) = false, want true (buildWorkspaceDescriptors must register the synthetic class)", syntheticSlug)
	}

	// Open the workspace-scoped "/" palette and confirm its single "Crear
	// ..." leaf (workspaceActions) — this is startCreateEditor's real entry
	// point, exactly as a user reaches it.
	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, enter())

	// classFilter is pre-seeded (design §5's startCreateEditor pre-seed/skip
	// trick), so the wizard starts one step later, directly at the Familia
	// question — its ONLY option must be the synthetic class's own family.
	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Instalación") {
		t.Fatalf("view after starting the create flow = %q, want the synthetic class's own family %q to render", plain, "Instalación")
	}
	m, _ = update(t, m, enter()) // select "Instalación"

	plain = ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Puesta en marcha") {
		t.Fatalf("view after selecting the family = %q, want the synthetic class's own type %q to render", plain, "Puesta en marcha")
	}
	m, _ = update(t, m, enter()) // select "Puesta en marcha"

	plain = ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Nivel de servicio") {
		t.Fatalf("view after selecting the type = %q, want the synthetic class's own attribute prompt %q to render", plain, "Nivel de servicio")
	}
	if !strings.Contains(plain, "Básico") || !strings.Contains(plain, "Avanzado") {
		t.Fatalf("view after selecting the type = %q, want the synthetic class's own option values (Básico/Avanzado) to be selectable", plain)
	}
	m, _ = update(t, m, enter()) // select "Básico"

	// Natural unit: the synthetic class's own UnitPolicy (PZA) must resolve.
	plain = ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "unidad natural") {
		t.Fatalf("view after answering the attribute = %q, want the natural-unit question", plain)
	}
	m, _ = update(t, m, enter()) // select the unit

	if creator.callCount != 1 {
		t.Fatalf("creator.callCount = %d, want 1 (the create flow must reach domain.NewResource/Create)", creator.callCount)
	}
	if creator.gotResource.ClassCode != "SERVICIO_ESPECIALIZADO" {
		t.Fatalf("created resource ClassCode = %q, want %q", creator.gotResource.ClassCode, "SERVICIO_ESPECIALIZADO")
	}
	if creator.gotResource.FamilyCode != "INSTALACION" || creator.gotResource.TypeCode != "PUESTA_EN_MARCHA" {
		t.Fatalf("created resource Family/Type = %q/%q, want %q/%q", creator.gotResource.FamilyCode, creator.gotResource.TypeCode, "INSTALACION", "PUESTA_EN_MARCHA")
	}
}
