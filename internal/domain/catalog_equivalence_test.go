package domain

import "testing"

// baseEquivalenceCatalog populates one instance of all 11 catalog kinds;
// subtests mutate a deep copy (cloneMutationCatalog) of it.
func baseEquivalenceCatalog() ResourceCatalog {
	return ResourceCatalog{
		Classes: []ResourceClass{{
			Code: "CABLE", Name: "Cable", Plural: "Cables", Slug: "cable", Order: 1, Active: true,
		}},
		Families: []ResourceFamily{{ClassCode: "CABLE", Code: "THHN", Name: "THHN", Active: true}},
		Types:    []ResourceType{{ClassCode: "CABLE", FamilyCode: "THHN", Code: "AWG12", Name: "AWG 12", Active: true}},
		Definitions: []AttributeDefinition{
			{Code: "color", Name: "Color", ValueType: ValueTypeControlledOption, Active: true},
		},
		OptionSets: []ResourceOptionSet{{Code: "DEFAULT", Name: "Default", Active: true}},
		Options: []AttributeOption{
			{OptionSet: "DEFAULT", AttributeCode: "color", Code: "ROJO", Label: "Rojo", Active: true},
			{OptionSet: "DEFAULT", AttributeCode: "color", Code: "AZUL", Label: "Azul", Active: true},
		},
		Relations: []AttributeOptionRelation{{
			OptionSet: "DEFAULT", FromAttribute: "color", FromOption: "ROJO", ToAttribute: "color", ToOption: "AZUL", Active: true,
		}},
		Units:        []UnitDefinition{{Code: "M", Name: "Metro", Symbol: "m", Dimension: "length", Active: true}},
		UnitPolicies: []ResourceUnitPolicy{{ClassCode: "CABLE", FamilyCode: "THHN", UnitCode: "M", Allowed: true, Suggested: true, Active: true}},
		Attributes: []ResourceAttribute{{
			ClassCode: "CABLE", FamilyCode: "THHN", TypeCode: "AWG12", OptionSet: "DEFAULT",
			Definition: AttributeDefinition{Code: "color"}, Mode: ModeConditional,
			Rules: []AttributeRule{
				{When: AttributeCondition{AttributeCode: "color", Equals: "ROJO"}, Mode: ModeRequired, Active: true},
				{When: AttributeCondition{AttributeCode: "color", Equals: "AZUL"}, Mode: ModeOptional, Active: true},
			},
			Active: true,
		}},
		PresentationFields: []PresentationField{{
			ClassCode: "CABLE", FamilyCode: "THHN", TypeCode: "AWG12", AttributeCode: "color", Position: 0, Active: true,
		}},
	}
}

type equivCase struct {
	name    string
	mutateB func(*ResourceCatalog)
}

func runEquivalenceCases(t *testing.T, wantEqual bool, cases []equivCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseEquivalenceCatalog()
			b := cloneMutationCatalog(a)
			tc.mutateB(&b)
			if got := EquivalentResourceCatalogs(a, b); got != wantEqual {
				t.Fatalf("EquivalentResourceCatalogs = %v, want %v: %s", got, wantEqual, tc.name)
			}
			if got := EquivalentResourceCatalogs(b, a); got != wantEqual {
				t.Fatalf("EquivalentResourceCatalogs (reversed) = %v, want %v: %s", got, wantEqual, tc.name)
			}
		})
	}
}

// TestCatalogEquivalenceApprovedNormalizations: identical deep-cloned
// catalogs plus the exactly-two approved normalizations (default
// option-set spelling, nil/empty collections) all compare EQUAL.
func TestCatalogEquivalenceApprovedNormalizations(t *testing.T) {
	runEquivalenceCases(t, true, []equivCase{
		{"identical deep-cloned catalog", func(c *ResourceCatalog) {}},
		{"nil vs non-nil-empty Aliases/Keywords", func(c *ResourceCatalog) {
			c.Classes[0].Aliases, c.Classes[0].Keywords = []string{}, []string{}
		}},
		{"nil vs non-nil-empty Rules", func(c *ResourceCatalog) {
			c.Attributes[0].Rules = append([]AttributeRule{}, c.Attributes[0].Rules...)
		}},
		{`"" OptionSet on AttributeOption == "DEFAULT"`, func(c *ResourceCatalog) { c.Options[0].OptionSet = "" }},
		{`"" OptionSet on AttributeOptionRelation == "DEFAULT"`, func(c *ResourceCatalog) { c.Relations[0].OptionSet = "" }},
		{`"" OptionSet on the ResourceAttribute binding == "DEFAULT"`, func(c *ResourceCatalog) { c.Attributes[0].OptionSet = "" }},
		{`"" CONJUNTO_OPCIONES Code == "DEFAULT"`, func(c *ResourceCatalog) { c.OptionSets[0].Code = "" }},
	})
}

// TestCatalogEquivalenceDetectsSemanticDifferences: everything outside the
// two approved normalizations stays UNEQUAL — a record present on only one
// side (field/rule omission, all 11 kinds), Active, identity, reference
// fields, and (TRIANGULATE) APLICABILIDAD rule order/content.
func TestCatalogEquivalenceDetectsSemanticDifferences(t *testing.T) {
	runEquivalenceCases(t, false, []equivCase{
		{"CLASE omitted", func(c *ResourceCatalog) { c.Classes = nil }},
		{"FAMILIA omitted", func(c *ResourceCatalog) { c.Families = nil }},
		{"TIPO omitted", func(c *ResourceCatalog) { c.Types = nil }},
		{"CARACTERISTICA omitted", func(c *ResourceCatalog) { c.Definitions = nil }},
		{"CONJUNTO_OPCIONES omitted", func(c *ResourceCatalog) { c.OptionSets = nil }},
		{"OPCION omitted", func(c *ResourceCatalog) { c.Options = c.Options[:1] }},
		{"RELACION_OPCIONES omitted", func(c *ResourceCatalog) { c.Relations = nil }},
		{"UNIDAD omitted", func(c *ResourceCatalog) { c.Units = nil }},
		{"POLITICA_UNIDAD omitted", func(c *ResourceCatalog) { c.UnitPolicies = nil }},
		{"APLICABILIDAD record omitted", func(c *ResourceCatalog) { c.Attributes = nil }},
		{"APLICABILIDAD rule omitted", func(c *ResourceCatalog) { c.Attributes[0].Rules = c.Attributes[0].Rules[:1] }},
		{"PRESENTACION omitted", func(c *ResourceCatalog) { c.PresentationFields = nil }},
		{"CLASE Active differs", func(c *ResourceCatalog) { c.Classes[0].Active = false }},
		{"CLASE identity (Code) differs", func(c *ResourceCatalog) { c.Classes[0].Code = "OTRO" }},
		{"FAMILIA reference (ClassCode) differs", func(c *ResourceCatalog) { c.Families[0].ClassCode = "OTRO" }},
		{"TIPO reference (FamilyCode) differs", func(c *ResourceCatalog) { c.Types[0].FamilyCode = "OTRO" }},
		{"APLICABILIDAD reference (Definition.Code) differs", func(c *ResourceCatalog) { c.Attributes[0].Definition.Code = "otro" }},
		{"APLICABILIDAD materialized Definition.Name differs (same Code)", func(c *ResourceCatalog) { c.Attributes[0].Definition.Name = "Color (stale)" }},
		{"RELACION_OPCIONES reference (FromAttribute) differs", func(c *ResourceCatalog) { c.Relations[0].FromAttribute = "otro" }},
		{"APLICABILIDAD rule order differs, same elements", func(c *ResourceCatalog) {
			r := c.Attributes[0].Rules
			r[0], r[1] = r[1], r[0]
		}},
		{"APLICABILIDAD rule content differs (Mode)", func(c *ResourceCatalog) { c.Attributes[0].Rules[0].Mode = ModeForbidden }},
		{"APLICABILIDAD rule content differs (When.Equals)", func(c *ResourceCatalog) { c.Attributes[0].Rules[0].When.Equals = "VERDE" }},
	})
}

// TestCatalogNormalizePureAndCopySafe (REFACTOR): EquivalentResourceCatalogs
// never mutates either input and shares no backing storage — mutating a
// after the call leaves b completely unaffected.
func TestCatalogNormalizePureAndCopySafe(t *testing.T) {
	a := baseEquivalenceCatalog()
	b := cloneMutationCatalog(a)
	if !EquivalentResourceCatalogs(a, b) {
		t.Fatal("expected the deep-cloned catalog to compare equal before mutation")
	}

	a.Options[0].Label = "MUTATED"
	a.Classes[0].Name = "Mutated Name"
	a.Attributes[0].Rules[0].Mode = ModeForbidden

	if b.Options[0].Label == "MUTATED" || b.Classes[0].Name == "Mutated Name" || b.Attributes[0].Rules[0].Mode == ModeForbidden {
		t.Fatal("EquivalentResourceCatalogs must not share backing storage between a and b")
	}
	if EquivalentResourceCatalogs(a, b) {
		t.Fatal("expected the mutated catalog to no longer compare equal")
	}
}
