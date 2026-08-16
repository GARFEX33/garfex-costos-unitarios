package domain

import "testing"

// Task 2b.4 (NEW): cross-class narrowing collision matrix per design's
// Testing Strategy item 2 and spec scenarios "Shared attribute code,
// different option sets" / "Explicit shared option set".
//
// TestResourceCatalog_OptionsFor_NarrowsByNamedOptionSet and
// TestResourceCatalog_OptionsFor_ExplicitSharedOptionSet
// (resource_catalog_query_test.go, task 2a.3) already proved this in
// isolation against OptionsFor. This file proves the SAME guarantee holds
// END-TO-END through NewResource's full validation pipeline (option
// canonicalization AND relation coherence) and that OptionsFor/ValidOptions
// agree — using a synthetic two-class fixture the real catalog does not
// yet have (real MANO_DE_OBRA/EQUIPO_HERRAMIENTA class seeding is PR3's
// job; PR3 should avoid reusing TEST_CLASS_A/TEST_CLASS_B as real class
// codes since this fixture is expected to remain in the suite).
//
// These are NEW test scenarios exercising already-correct PR2a production
// code (ResourceScope/OptionSet class-scoping and canonicalOptionSet), so
// RED could not be forced without artificially breaking working code; per
// session instructions this is the "genuinely new scenario against
// already-correct code" case — see this PR's TDD Cycle Evidence table.

func twoClassOptionSetCatalog() ResourceCatalog {
	return ResourceCatalog{
		Classes: []ResourceClass{
			{Code: "TEST_CLASS_A", Name: "Test Class A", Plural: "Test Classes A", Slug: "test-class-a", Order: 1, Active: true},
			{Code: "TEST_CLASS_B", Name: "Test Class B", Plural: "Test Classes B", Slug: "test-class-b", Order: 2, Active: true},
		},
		Families: []ResourceFamily{
			{ClassCode: "TEST_CLASS_A", Code: "WIDGETS", Name: "Widgets A", Active: true},
			{ClassCode: "TEST_CLASS_B", Code: "WIDGETS", Name: "Widgets B", Active: true},
		},
		Types: []ResourceType{
			{ClassCode: "TEST_CLASS_A", FamilyCode: "WIDGETS", Code: "WIDGET", Name: "Widget A", Active: true},
			{ClassCode: "TEST_CLASS_B", FamilyCode: "WIDGETS", Code: "WIDGET", Name: "Widget B", Active: true},
		},
		Units: []UnitDefinition{{Code: "PZA", Name: "Pieza", Symbol: "PZA", Dimension: "PIECE", Active: true}},
		UnitPolicies: []ResourceUnitPolicy{
			{ClassCode: "TEST_CLASS_A", FamilyCode: "WIDGETS", UnitCode: "PZA", Allowed: true, Suggested: true},
			{ClassCode: "TEST_CLASS_B", FamilyCode: "WIDGETS", UnitCode: "PZA", Allowed: true, Suggested: true},
		},
		Definitions: []AttributeDefinition{
			{Code: "color", Name: "Color", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
			{Code: "size", Name: "Size", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
		},
		Attributes: []ResourceAttribute{
			{ClassCode: "TEST_CLASS_A", FamilyCode: "WIDGETS", TypeCode: "WIDGET", OptionSet: "COLOR_A", Definition: AttributeDefinition{Code: "color", ValueType: ValueTypeControlledOption}, Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "TEST_CLASS_B", FamilyCode: "WIDGETS", TypeCode: "WIDGET", OptionSet: "COLOR_B", Definition: AttributeDefinition{Code: "color", ValueType: ValueTypeControlledOption}, Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "TEST_CLASS_A", FamilyCode: "WIDGETS", TypeCode: "WIDGET", OptionSet: "SHARED", Definition: AttributeDefinition{Code: "size", ValueType: ValueTypeControlledOption}, Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "TEST_CLASS_B", FamilyCode: "WIDGETS", TypeCode: "WIDGET", OptionSet: "SHARED", Definition: AttributeDefinition{Code: "size", ValueType: ValueTypeControlledOption}, Mode: ModeRequired, IdentityParticipates: true},
		},
		Options: []AttributeOption{
			{OptionSet: "COLOR_A", AttributeCode: "color", Code: "ROJO", Label: "Rojo", Active: true},
			{OptionSet: "COLOR_B", AttributeCode: "color", Code: "AZUL", Label: "Azul", Active: true},
			{OptionSet: "SHARED", AttributeCode: "size", Code: "CHICO", Label: "Chico", Active: true},
			{OptionSet: "SHARED", AttributeCode: "size", Code: "GRANDE", Label: "Grande", Active: true},
		},
	}
}

// Spec "Shared attribute code, different option sets" — end-to-end through
// NewResource: class A's "color" only ever accepts COLOR_A's own options,
// never class B's.
func TestNewResource_RejectsOptionFromSiblingClassDifferentOptionSet(t *testing.T) {
	catalog := twoClassOptionSetCatalog()
	scope := ResourceScope{ClassCode: "TEST_CLASS_A", FamilyCode: "WIDGETS", TypeCode: "WIDGET"}
	if _, err := NewResource(catalog, scope, "PZA", []ResourceAttributeValue{OptionValue("color", "ROJO"), OptionValue("size", "CHICO")}); err != nil {
		t.Fatalf("NewResource(class A, color=ROJO — its own option) error = %v, want nil", err)
	}
	if _, err := NewResource(catalog, scope, "PZA", []ResourceAttributeValue{OptionValue("color", "AZUL"), OptionValue("size", "CHICO")}); err == nil {
		t.Fatal("NewResource(class A, color=AZUL — class B's option) error = nil, want validation error")
	}
}

// The mirror direction: class B never accepts class A's COLOR_A options.
func TestNewResource_RejectsOptionFromSiblingClassReverseDirection(t *testing.T) {
	catalog := twoClassOptionSetCatalog()
	scope := ResourceScope{ClassCode: "TEST_CLASS_B", FamilyCode: "WIDGETS", TypeCode: "WIDGET"}
	if _, err := NewResource(catalog, scope, "PZA", []ResourceAttributeValue{OptionValue("color", "AZUL"), OptionValue("size", "CHICO")}); err != nil {
		t.Fatalf("NewResource(class B, color=AZUL — its own option) error = %v, want nil", err)
	}
	if _, err := NewResource(catalog, scope, "PZA", []ResourceAttributeValue{OptionValue("color", "ROJO"), OptionValue("size", "CHICO")}); err == nil {
		t.Fatal("NewResource(class B, color=ROJO — class A's option) error = nil, want validation error")
	}
}

// Spec "Explicit shared option set" — end-to-end: two classes' "size"
// attribute both name OptionSet "SHARED" and correctly accept the SAME
// option codes.
func TestNewResource_AcceptsExplicitlySharedOptionSetAcrossClasses(t *testing.T) {
	catalog := twoClassOptionSetCatalog()
	for _, tt := range []struct {
		class string
		color string
	}{{"TEST_CLASS_A", "ROJO"}, {"TEST_CLASS_B", "AZUL"}} {
		scope := ResourceScope{ClassCode: tt.class, FamilyCode: "WIDGETS", TypeCode: "WIDGET"}
		if _, err := NewResource(catalog, scope, "PZA", []ResourceAttributeValue{OptionValue("color", tt.color), OptionValue("size", "GRANDE")}); err != nil {
			t.Fatalf("NewResource(%s, size=GRANDE — shared option) error = %v, want nil", tt.class, err)
		}
	}
}

// Collision-matrix check: sharing an attribute CODE across classes with
// DIFFERENT option sets must never leak the sibling's options, on EITHER
// query surface — OptionsFor and ValidOptions(nil) must agree.
func TestOptionSetIsolation_OptionsForAndValidOptionsAgree(t *testing.T) {
	catalog := twoClassOptionSetCatalog()
	classAColor := findResourceAttribute(t, catalog.AttributesFor(ResourceScope{ClassCode: "TEST_CLASS_A", FamilyCode: "WIDGETS", TypeCode: "WIDGET"}), "color")
	classBColor := findResourceAttribute(t, catalog.AttributesFor(ResourceScope{ClassCode: "TEST_CLASS_B", FamilyCode: "WIDGETS", TypeCode: "WIDGET"}), "color")

	for _, tt := range []struct {
		name      string
		attribute ResourceAttribute
		wantCode  string
	}{{"class A", classAColor, "ROJO"}, {"class B", classBColor, "AZUL"}} {
		t.Run(tt.name, func(t *testing.T) {
			byOptionsFor := catalog.OptionsFor(tt.attribute)
			byValidOptions := catalog.ValidOptions(tt.attribute, nil)
			if len(byOptionsFor) != 1 || byOptionsFor[0].Code != tt.wantCode {
				t.Fatalf("OptionsFor = %+v, want only %q", byOptionsFor, tt.wantCode)
			}
			if len(byValidOptions) != 1 || byValidOptions[0].Code != tt.wantCode {
				t.Fatalf("ValidOptions(nil) = %+v, want only %q", byValidOptions, tt.wantCode)
			}
		})
	}
}
