package domain

import "testing"

// TestAttributesForReturnsCatalogDeclarationOrder covers AttributesFor's
// core contract: CONDUCTORES/CABLE's FamilyAttributes come back in exactly
// the order NewMaterialsCatalog() declares them (conductor_material, gauge,
// insulation, color, voltage) — the order a UI should ask about them in.
func TestAttributesForReturnsCatalogDeclarationOrder(t *testing.T) {
	catalog := NewMaterialsCatalog()
	attributes := catalog.AttributesFor("CONDUCTORES", "CABLE")
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

// TestAttributesForUnknownFamilyOrProductTypeReturnsEmpty covers
// AttributesFor's read-query contract: an unknown family/productType yields
// an empty result, not an error — validation is NewMaterial's job.
func TestAttributesForUnknownFamilyOrProductTypeReturnsEmpty(t *testing.T) {
	catalog := NewMaterialsCatalog()
	if got := catalog.AttributesFor("NOPE", "CABLE"); len(got) != 0 {
		t.Fatalf("AttributesFor(unknown family) = %v, want empty", got)
	}
	if got := catalog.AttributesFor("CONDUCTORES", "NOPE"); len(got) != 0 {
		t.Fatalf("AttributesFor(unknown productType) = %v, want empty", got)
	}
}

// TestOptionsForReturnsColorOptions covers OptionsFor's core contract: the
// real catalog's 5 color options come back in catalog declaration order.
func TestOptionsForReturnsColorOptions(t *testing.T) {
	catalog := NewMaterialsCatalog()
	options := catalog.OptionsFor("color")
	want := []string{"NEGRO", "BLANCO", "ROJO", "AZUL", "VERDE"}
	if len(options) != len(want) {
		t.Fatalf("OptionsFor(\"color\") returned %d options, want %d", len(options), len(want))
	}
	for i, code := range want {
		if options[i].Code != code {
			t.Fatalf("OptionsFor(\"color\")[%d].Code = %q, want %q", i, options[i].Code, code)
		}
	}
}

// TestOptionsForAttributeWithNoOptionsReturnsEmpty covers OptionsFor for an
// attribute that has no AttributeOption entries at all (voltage is a
// QUANTITY, not a CONTROLLED_OPTION) and for an unknown attribute code.
func TestOptionsForAttributeWithNoOptionsReturnsEmpty(t *testing.T) {
	catalog := NewMaterialsCatalog()
	if got := catalog.OptionsFor("voltage"); len(got) != 0 {
		t.Fatalf("OptionsFor(\"voltage\") = %v, want empty", got)
	}
	if got := catalog.OptionsFor("nope"); len(got) != 0 {
		t.Fatalf("OptionsFor(\"nope\") = %v, want empty", got)
	}
}

// TestNaturalUnitsForRealCatalogReturnsSingleSuggestedUnit covers
// NaturalUnitsFor against the real catalog: CONDUCTORES allows exactly one
// unit (M), which is also its suggested unit.
func TestNaturalUnitsForRealCatalogReturnsSingleSuggestedUnit(t *testing.T) {
	catalog := NewMaterialsCatalog()
	units := catalog.NaturalUnitsFor("CONDUCTORES")
	if len(units) != 1 || units[0].Code != "M" {
		t.Fatalf("NaturalUnitsFor(\"CONDUCTORES\") = %v, want [M]", units)
	}
}

// TestNaturalUnitsForOrdersSuggestedFirstRegardlessOfDeclarationOrder covers
// NaturalUnitsFor's ordering contract with a local catalog literal: the
// suggested unit comes first in the result even when it is declared second
// in UnitPolicies.
func TestNaturalUnitsForOrdersSuggestedFirstRegardlessOfDeclarationOrder(t *testing.T) {
	catalog := MaterialsCatalog{
		Units: []UnitDefinition{
			{Code: "PZA", Symbol: "PZA", Dimension: "PIECE"},
			{Code: "M", Symbol: "M", Dimension: "LENGTH"},
		},
		UnitPolicies: []FamilyUnitPolicy{
			{FamilyCode: "WIDGETS", UnitCode: "PZA", Allowed: true, Suggested: false},
			{FamilyCode: "WIDGETS", UnitCode: "M", Allowed: true, Suggested: true},
		},
	}
	units := catalog.NaturalUnitsFor("WIDGETS")
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
// core new logic: given diameter_inch=1/2" already chosen, diameter_mm
// narrows to exactly its related option (13 mm), using the real
// tuberiasRelations() pairing.
func TestValidOptionsNarrowsDiameterMmByChosenDiameterInch(t *testing.T) {
	catalog := NewMaterialsCatalog()
	current := []MaterialAttributeValue{OptionValue("diameter_inch", `1/2"`)}
	options := catalog.ValidOptions("diameter_mm", current)
	if len(options) != 1 || options[0].Code != "13 mm" {
		t.Fatalf("ValidOptions(\"diameter_mm\", ...) = %v, want [13 mm]", options)
	}
}

// TestValidOptionsNarrowsDiameterInchByChosenDiameterMm covers the reverse
// direction: AttributeOptionRelation is stored directionally
// (diameter_inch -> diameter_mm) but ValidOptions must narrow both ways.
func TestValidOptionsNarrowsDiameterInchByChosenDiameterMm(t *testing.T) {
	catalog := NewMaterialsCatalog()
	current := []MaterialAttributeValue{OptionValue("diameter_mm", "13 mm")}
	options := catalog.ValidOptions("diameter_inch", current)
	if len(options) != 1 || options[0].Code != `1/2"` {
		t.Fatalf("ValidOptions(\"diameter_inch\", ...) = %v, want [1/2\"]", options)
	}
}

// TestValidOptionsWithNothingChosenReturnsAllUnconstrained covers
// ValidOptions when current is nil: no relation can be satisfied, so every
// real catalog diameter_mm option (9 total) comes back unconstrained.
func TestValidOptionsWithNothingChosenReturnsAllUnconstrained(t *testing.T) {
	catalog := NewMaterialsCatalog()
	options := catalog.ValidOptions("diameter_mm", nil)
	want := catalog.OptionsFor("diameter_mm")
	if len(options) != 9 {
		t.Fatalf("ValidOptions(\"diameter_mm\", nil) returned %d options, want 9", len(options))
	}
	if len(options) != len(want) {
		t.Fatalf("ValidOptions(\"diameter_mm\", nil) = %v, want %v", options, want)
	}
	for i := range want {
		if options[i].Code != want[i].Code {
			t.Fatalf("ValidOptions(\"diameter_mm\", nil)[%d].Code = %q, want %q", i, options[i].Code, want[i].Code)
		}
	}
}

// TestValidOptionsWithNoRelationsReturnsFullListRegardlessOfCurrent covers
// ValidOptions for an attribute that no AttributeOptionRelation touches at
// all (color): it always returns its full unconstrained option list, no
// matter what current holds.
func TestValidOptionsWithNoRelationsReturnsFullListRegardlessOfCurrent(t *testing.T) {
	catalog := NewMaterialsCatalog()
	want := catalog.OptionsFor("color")
	current := []MaterialAttributeValue{
		OptionValue("insulation", "DESNUDO"),
		OptionValue("diameter_inch", `1/2"`),
	}
	options := catalog.ValidOptions("color", current)
	if len(options) != len(want) {
		t.Fatalf("ValidOptions(\"color\", ...) returned %d options, want %d", len(options), len(want))
	}
	for i := range want {
		if options[i].Code != want[i].Code {
			t.Fatalf("ValidOptions(\"color\", ...)[%d].Code = %q, want %q", i, options[i].Code, want[i].Code)
		}
	}
}

// TestFamilyAttributeEffectiveAppliesConditionalRule covers
// FamilyAttribute.Effective for CONDUCTORES/CABLE's color attribute: when
// insulation=DESNUDO is already chosen, its ModeConditional rule fires and
// resolves to ModeForbidden/notApplicable=true — mirroring what NewMaterial
// already enforces internally.
func TestFamilyAttributeEffectiveAppliesConditionalRule(t *testing.T) {
	catalog := NewMaterialsCatalog()
	color := findFamilyAttribute(t, catalog.AttributesFor("CONDUCTORES", "CABLE"), "color")
	mode, _, notApplicable := color.Effective([]MaterialAttributeValue{OptionValue("insulation", "DESNUDO")})
	if mode != ModeForbidden || !notApplicable {
		t.Fatalf("Effective(insulation=DESNUDO) = (%v, notApplicable=%v), want (ModeForbidden, true)", mode, notApplicable)
	}
}

// TestFamilyAttributeEffectiveFallsBackToRequiredWhenNoRuleMatches covers
// FamilyAttribute.Effective's fallback: when insulation=THW does not match
// color's DESNUDO-only rule, ModeConditional falls through to ModeRequired
// (mirroring the existing private effective()'s unmatched-rule fallback).
func TestFamilyAttributeEffectiveFallsBackToRequiredWhenNoRuleMatches(t *testing.T) {
	catalog := NewMaterialsCatalog()
	color := findFamilyAttribute(t, catalog.AttributesFor("CONDUCTORES", "CABLE"), "color")
	mode, participates, notApplicable := color.Effective([]MaterialAttributeValue{OptionValue("insulation", "THW")})
	if mode != ModeRequired || notApplicable {
		t.Fatalf("Effective(insulation=THW) = (%v, notApplicable=%v), want (ModeRequired, false)", mode, notApplicable)
	}
	if !participates {
		t.Fatalf("Effective(insulation=THW) identityParticipates = false, want true")
	}
}

func findFamilyAttribute(t *testing.T, attributes []FamilyAttribute, code string) FamilyAttribute {
	t.Helper()
	for _, attribute := range attributes {
		if attribute.Definition.Code == code {
			return attribute
		}
	}
	t.Fatalf("no FamilyAttribute with Definition.Code %q", code)
	return FamilyAttribute{}
}
