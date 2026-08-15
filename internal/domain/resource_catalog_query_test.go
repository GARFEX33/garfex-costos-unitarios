package domain

import "testing"

// RED (task 2a.1): ResourceScope-based lookups must reject cross-class
// family/type reuse. Both cases fail to compile until 2a.2's GREEN lands
// SeedResourceCatalog()/hasFamily(ResourceScope)/hasType(ResourceScope) —
// the mechanical-rename-slice exception to RED-via-compiler (design
// Testing Strategy). PR2b later extends this same file with the full
// adapted table-case coverage from material_catalog_query_test.go.

// Spec scenario "Family valid only within its class".
func TestResourceCatalog_hasFamily_ScopedByClass(t *testing.T) {
	catalog := SeedResourceCatalog()
	if !catalog.hasFamily(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES"}) {
		t.Fatalf("hasFamily(MATERIAL/CONDUCTORES) = false, want true")
	}
	if catalog.hasFamily(ResourceScope{ClassCode: "MANO_DE_OBRA", FamilyCode: "CONDUCTORES"}) {
		t.Fatalf("hasFamily(MANO_DE_OBRA/CONDUCTORES) = true, want false — CONDUCTORES is owned by MATERIAL, not MANO_DE_OBRA")
	}
}

// Spec scenario "Type valid only within its family+class".
func TestResourceCatalog_hasType_ScopedByFamily(t *testing.T) {
	catalog := SeedResourceCatalog()
	if !catalog.hasType(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}) {
		t.Fatalf("hasType(MATERIAL/CONDUCTORES/CABLE) = false, want true")
	}
	if catalog.hasType(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "CABLE"}) {
		t.Fatalf("hasType(MATERIAL/CANALIZACIONES/CABLE) = true, want false — CABLE is owned by family CONDUCTORES, not CANALIZACIONES")
	}
}

// RED (task 2a.3): named OptionSet narrowing. Spec scenario "Shared
// attribute code, different option sets".
func TestResourceCatalog_OptionsFor_NarrowsByNamedOptionSet(t *testing.T) {
	catalog := ResourceCatalog{
		Attributes: []ResourceAttribute{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Definition: AttributeDefinition{Code: "color"}, OptionSet: "COLORES_CONDUCTOR"},
			{ClassCode: "EQUIPO_HERRAMIENTA", FamilyCode: "HERRAMIENTA_MANUAL", Definition: AttributeDefinition{Code: "color"}, OptionSet: "COLORES_EQUIPO"},
		},
		Options: []AttributeOption{
			{OptionSet: "COLORES_CONDUCTOR", AttributeCode: "color", Code: "NEGRO", Label: "Negro"},
			{OptionSet: "COLORES_EQUIPO", AttributeCode: "color", Code: "AMARILLO", Label: "Amarillo"},
		},
	}
	materialColor, equipoColor := catalog.Attributes[0], catalog.Attributes[1]

	materialOptions := catalog.OptionsFor(materialColor)
	if len(materialOptions) != 1 || materialOptions[0].Code != "NEGRO" {
		t.Fatalf("OptionsFor(material color) = %+v, want only NEGRO", materialOptions)
	}
	equipoOptions := catalog.OptionsFor(equipoColor)
	if len(equipoOptions) != 1 || equipoOptions[0].Code != "AMARILLO" {
		t.Fatalf("OptionsFor(equipo color) = %+v, want only AMARILLO", equipoOptions)
	}
}

// Spec scenario "Explicit shared option set".
func TestResourceCatalog_OptionsFor_ExplicitSharedOptionSet(t *testing.T) {
	catalog := ResourceCatalog{
		Attributes: []ResourceAttribute{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Definition: AttributeDefinition{Code: "color"}, OptionSet: "COLORES_BASICOS"},
			{ClassCode: "EQUIPO_HERRAMIENTA", FamilyCode: "HERRAMIENTA_MANUAL", Definition: AttributeDefinition{Code: "color"}, OptionSet: "COLORES_BASICOS"},
		},
		Options: []AttributeOption{
			{OptionSet: "COLORES_BASICOS", AttributeCode: "color", Code: "NEGRO", Label: "Negro"},
			{OptionSet: "COLORES_BASICOS", AttributeCode: "color", Code: "ROJO", Label: "Rojo"},
		},
	}
	a := catalog.OptionsFor(catalog.Attributes[0])
	b := catalog.OptionsFor(catalog.Attributes[1])
	if len(a) != 2 || len(b) != 2 || a[0].Code != b[0].Code || a[1].Code != b[1].Code {
		t.Fatalf("OptionsFor for two classes explicitly sharing COLORES_BASICOS = %+v vs %+v, want identical", a, b)
	}
}

// --- Task 2b.1: adapted from the pre-rename material_catalog_query_test.go
// (14 table cases), against ResourceScope/ResourceAttribute call shapes.
// Mechanical rename slice: the compiler failing to build internal/domain_test
// was the RED signal (session instructions) — no new behavior to RED
// against, since PR2a's production code already implements every one of
// these contracts under its new names/shapes.

func findResourceAttribute(t *testing.T, attributes []ResourceAttribute, code string) ResourceAttribute {
	t.Helper()
	for _, attribute := range attributes {
		if attribute.Definition.Code == code {
			return attribute
		}
	}
	t.Fatalf("no ResourceAttribute with Definition.Code %q", code)
	return ResourceAttribute{}
}

// TestAttributesForReturnsCatalogDeclarationOrder covers AttributesFor's
// core contract: CONDUCTORES/CABLE's attributes come back in exactly the
// order SeedResourceCatalog() declares them.
func TestAttributesForReturnsCatalogDeclarationOrder(t *testing.T) {
	catalog := SeedResourceCatalog()
	attributes := catalog.AttributesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"})
	want := []string{"conductor_material", "gauge", "insulation", "color", "voltage"}
	if len(attributes) != len(want) {
		t.Fatalf("AttributesFor() returned %d attributes, want %d", len(attributes), len(want))
	}
	for i, code := range want {
		if attributes[i].Definition.Code != code {
			t.Fatalf("AttributesFor()[%d].Definition.Code = %q, want %q", i, attributes[i].Definition.Code, code)
		}
	}
}

// TestAttributesForUnknownFamilyOrTypeReturnsEmpty covers AttributesFor's
// read-query contract: an unknown family/type yields an empty result, not
// an error — validation is NewResource's job.
func TestAttributesForUnknownFamilyOrTypeReturnsEmpty(t *testing.T) {
	catalog := SeedResourceCatalog()
	if got := catalog.AttributesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "NOPE", TypeCode: "CABLE"}); len(got) != 0 {
		t.Fatalf("AttributesFor(unknown family) = %v, want empty", got)
	}
	if got := catalog.AttributesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "NOPE"}); len(got) != 0 {
		t.Fatalf("AttributesFor(unknown type) = %v, want empty", got)
	}
}

// TestOptionsForReturnsColorOptions covers OptionsFor's core contract: the
// real catalog's 5 color options come back in catalog declaration order.
func TestOptionsForReturnsColorOptions(t *testing.T) {
	catalog := SeedResourceCatalog()
	color := findResourceAttribute(t, catalog.AttributesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}), "color")
	options := catalog.OptionsFor(color)
	want := []string{"NEGRO", "BLANCO", "ROJO", "AZUL", "VERDE"}
	if len(options) != len(want) {
		t.Fatalf("OptionsFor(color) returned %d options, want %d", len(options), len(want))
	}
	for i, code := range want {
		if options[i].Code != code {
			t.Fatalf("OptionsFor(color)[%d].Code = %q, want %q", i, options[i].Code, code)
		}
	}
}

// TestOptionsForAttributeWithNoOptionsReturnsEmpty covers OptionsFor for an
// unknown attribute code, which has no AttributeOption entries at all.
func TestOptionsForAttributeWithNoOptionsReturnsEmpty(t *testing.T) {
	catalog := SeedResourceCatalog()
	if got := catalog.OptionsFor(ResourceAttribute{Definition: AttributeDefinition{Code: "nope"}}); len(got) != 0 {
		t.Fatalf("OptionsFor(unknown attribute) = %v, want empty", got)
	}
}

// TestNaturalUnitsForRealCatalogReturnsSingleSuggestedUnit covers
// NaturalUnitsFor against the real catalog: CONDUCTORES allows exactly one
// unit (M), which is also its suggested unit.
func TestNaturalUnitsForRealCatalogReturnsSingleSuggestedUnit(t *testing.T) {
	catalog := SeedResourceCatalog()
	units := catalog.NaturalUnitsFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES"})
	if len(units) != 1 || units[0].Code != "M" {
		t.Fatalf("NaturalUnitsFor(CONDUCTORES) = %v, want [M]", units)
	}
}

// TestNaturalUnitsForOrdersSuggestedFirstRegardlessOfDeclarationOrder covers
// NaturalUnitsFor's ordering contract with a local catalog literal: the
// suggested unit comes first in the result even when it is declared second
// in UnitPolicies.
func TestNaturalUnitsForOrdersSuggestedFirstRegardlessOfDeclarationOrder(t *testing.T) {
	catalog := ResourceCatalog{
		Units: []UnitDefinition{
			{Code: "PZA", Symbol: "PZA", Dimension: "PIECE"},
			{Code: "M", Symbol: "M", Dimension: "LENGTH"},
		},
		UnitPolicies: []ResourceUnitPolicy{
			{ClassCode: "MATERIAL", FamilyCode: "WIDGETS", UnitCode: "PZA", Allowed: true, Suggested: false},
			{ClassCode: "MATERIAL", FamilyCode: "WIDGETS", UnitCode: "M", Allowed: true, Suggested: true},
		},
	}
	units := catalog.NaturalUnitsFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "WIDGETS"})
	want := []string{"M", "PZA"}
	if len(units) != len(want) {
		t.Fatalf("NaturalUnitsFor() returned %d units, want %d", len(units), len(want))
	}
	for i, code := range want {
		if units[i].Code != code {
			t.Fatalf("NaturalUnitsFor()[%d].Code = %q, want %q", i, units[i].Code, code)
		}
	}
}

// TestValidOptionsNarrowsDiameterMmByChosenDiameterInch covers ValidOptions'
// core logic: given diameter_inch=1/2" already chosen, diameter_mm narrows
// to exactly its related option (13 mm), using the real tuberiasRelations()
// pairing.
func TestValidOptionsNarrowsDiameterMmByChosenDiameterInch(t *testing.T) {
	catalog := SeedResourceCatalog()
	scope := ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA"}
	diameterMM := findResourceAttribute(t, catalog.AttributesFor(scope), "diameter_mm")
	current := []ResourceAttributeValue{OptionValue("diameter_inch", `1/2"`)}
	options := catalog.ValidOptions(diameterMM, current)
	if len(options) != 1 || options[0].Code != "13 mm" {
		t.Fatalf("ValidOptions(diameter_mm, ...) = %v, want [13 mm]", options)
	}
}

// TestValidOptionsNarrowsDiameterInchByChosenDiameterMm covers the reverse
// direction: AttributeOptionRelation is stored directionally
// (diameter_inch -> diameter_mm) but ValidOptions must narrow both ways.
func TestValidOptionsNarrowsDiameterInchByChosenDiameterMm(t *testing.T) {
	catalog := SeedResourceCatalog()
	scope := ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA"}
	diameterInch := findResourceAttribute(t, catalog.AttributesFor(scope), "diameter_inch")
	current := []ResourceAttributeValue{OptionValue("diameter_mm", "13 mm")}
	options := catalog.ValidOptions(diameterInch, current)
	if len(options) != 1 || options[0].Code != `1/2"` {
		t.Fatalf(`ValidOptions(diameter_inch, ...) = %v, want [1/2"]`, options)
	}
}

// TestValidOptionsWithNothingChosenReturnsAllUnconstrained covers
// ValidOptions when current is nil: no relation can be satisfied, so every
// real catalog diameter_mm option (9 total) comes back unconstrained.
func TestValidOptionsWithNothingChosenReturnsAllUnconstrained(t *testing.T) {
	catalog := SeedResourceCatalog()
	scope := ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA"}
	diameterMM := findResourceAttribute(t, catalog.AttributesFor(scope), "diameter_mm")
	options := catalog.ValidOptions(diameterMM, nil)
	want := catalog.OptionsFor(diameterMM)
	if len(options) != 9 {
		t.Fatalf("ValidOptions(diameter_mm, nil) returned %d options, want 9", len(options))
	}
	if len(options) != len(want) {
		t.Fatalf("ValidOptions(diameter_mm, nil) = %v, want %v", options, want)
	}
	for i := range want {
		if options[i].Code != want[i].Code {
			t.Fatalf("ValidOptions(diameter_mm, nil)[%d].Code = %q, want %q", i, options[i].Code, want[i].Code)
		}
	}
}

// TestValidOptionsWithNoRelationsReturnsFullListRegardlessOfCurrent covers
// ValidOptions for an attribute that no AttributeOptionRelation touches at
// all (color): it always returns its full unconstrained option list, no
// matter what current holds.
func TestValidOptionsWithNoRelationsReturnsFullListRegardlessOfCurrent(t *testing.T) {
	catalog := SeedResourceCatalog()
	color := findResourceAttribute(t, catalog.AttributesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}), "color")
	want := catalog.OptionsFor(color)
	current := []ResourceAttributeValue{
		OptionValue("insulation", "DESNUDO"),
		OptionValue("diameter_inch", `1/2"`),
	}
	options := catalog.ValidOptions(color, current)
	if len(options) != len(want) {
		t.Fatalf("ValidOptions(color, ...) returned %d options, want %d", len(options), len(want))
	}
	for i := range want {
		if options[i].Code != want[i].Code {
			t.Fatalf("ValidOptions(color, ...)[%d].Code = %q, want %q", i, options[i].Code, want[i].Code)
		}
	}
}

// TestTypesForReturnsCatalogDeclarationOrder covers TypesFor's core
// contract against the real catalog: each family has exactly one type
// today.
func TestTypesForReturnsCatalogDeclarationOrder(t *testing.T) {
	catalog := SeedResourceCatalog()
	if got := catalog.TypesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES"}); len(got) != 1 || got[0].Code != "CABLE" {
		t.Fatalf("TypesFor(CONDUCTORES) = %v, want [CABLE]", got)
	}
	if got := catalog.TypesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES"}); len(got) != 1 || got[0].Code != "TUBERIA" {
		t.Fatalf("TypesFor(CANALIZACIONES) = %v, want [TUBERIA]", got)
	}
}

// TestTypesForUnknownFamilyReturnsEmpty covers TypesFor's read-query
// contract: an unknown family yields an empty result, not an error.
func TestTypesForUnknownFamilyReturnsEmpty(t *testing.T) {
	catalog := SeedResourceCatalog()
	if got := catalog.TypesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "NOPE"}); len(got) != 0 {
		t.Fatalf("TypesFor(unknown family) = %v, want empty", got)
	}
}

// TestResourceAttributeEffectiveAppliesConditionalRule covers
// ResourceAttribute.Effective for CONDUCTORES/CABLE's color attribute: when
// insulation=DESNUDO is already chosen, its ModeConditional rule fires and
// resolves to ModeForbidden/notApplicable=true — mirroring what NewResource
// already enforces internally.
func TestResourceAttributeEffectiveAppliesConditionalRule(t *testing.T) {
	catalog := SeedResourceCatalog()
	color := findResourceAttribute(t, catalog.AttributesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}), "color")
	mode, _, notApplicable := color.Effective([]ResourceAttributeValue{OptionValue("insulation", "DESNUDO")})
	if mode != ModeForbidden || !notApplicable {
		t.Fatalf("Effective(insulation=DESNUDO) = (%v, notApplicable=%v), want (ModeForbidden, true)", mode, notApplicable)
	}
}

// TestResourceAttributeEffectiveFallsBackToRequiredWhenNoRuleMatches covers
// ResourceAttribute.Effective's fallback: when insulation=THW does not
// match color's DESNUDO-only rule, ModeConditional falls through to
// ModeRequired (mirroring the existing private effective()'s
// unmatched-rule fallback).
func TestResourceAttributeEffectiveFallsBackToRequiredWhenNoRuleMatches(t *testing.T) {
	catalog := SeedResourceCatalog()
	color := findResourceAttribute(t, catalog.AttributesFor(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}), "color")
	mode, participates, notApplicable := color.Effective([]ResourceAttributeValue{OptionValue("insulation", "THW")})
	if mode != ModeRequired || notApplicable {
		t.Fatalf("Effective(insulation=THW) = (%v, notApplicable=%v), want (ModeRequired, false)", mode, notApplicable)
	}
	if !participates {
		t.Fatalf("Effective(insulation=THW) identityParticipates = false, want true")
	}
}
