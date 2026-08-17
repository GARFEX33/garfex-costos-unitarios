package domain

import (
	"errors"
	"testing"
)

// TestSeedResourceCatalogAllEntriesActive is the landmine guard (tasks
// reconciliation carve-out #3): adding Active bool to ResourceFamily/
// ResourceType/AttributeOption/UnitDefinition does NOT break
// SeedResourceCatalog()'s existing named-field struct literals — they
// silently default to Active: false, which FamiliesFor/TypesFor/OptionsFor/
// NaturalUnitsFor's new filtering (a later change in this same PR) would
// then silently hide. This test asserts every seeded Family/Type/Option/
// Unit is Active, so an omitted `Active: true` on any literal fails loudly
// here instead of silently emptying the production catalog.
func TestSeedResourceCatalogAllEntriesActive(t *testing.T) {
	catalog := SeedResourceCatalog()

	if len(catalog.Families) == 0 || len(catalog.Types) == 0 || len(catalog.Options) == 0 || len(catalog.Units) == 0 {
		t.Fatalf("SeedResourceCatalog() returned an empty catalog slice — fixture is broken, cannot prove the Active invariant")
	}

	for _, family := range catalog.Families {
		if !family.Active {
			t.Errorf("Family %s/%s has Active = false, want true", family.ClassCode, family.Code)
		}
	}
	for _, typ := range catalog.Types {
		if !typ.Active {
			t.Errorf("Type %s/%s/%s has Active = false, want true", typ.ClassCode, typ.FamilyCode, typ.Code)
		}
	}
	for _, option := range catalog.Options {
		if !option.Active {
			t.Errorf("Option %s/%s (set %q) has Active = false, want true", option.AttributeCode, option.Code, option.OptionSet)
		}
	}
	for _, unit := range catalog.Units {
		if !unit.Active {
			t.Errorf("Unit %s has Active = false, want true", unit.Code)
		}
	}
}

func TestNewResourceRejectsEveryInactiveWriteDependency(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*ResourceCatalog)
	}{
		{"class", func(c *ResourceCatalog) { c.Classes[0].Active = false }},
		{"family", func(c *ResourceCatalog) { c.Families[0].Active = false }},
		{"type", func(c *ResourceCatalog) { c.Types[0].Active = false }},
		{"unit", func(c *ResourceCatalog) { c.Units[0].Active = false }},
		{"unit policy", func(c *ResourceCatalog) { c.UnitPolicies[0].Active = false }},
		{"attribute binding", func(c *ResourceCatalog) { c.Attributes[0].Active = false }},
		{"attribute definition", func(c *ResourceCatalog) { c.Definitions[0].Active = false }},
		{"option set", func(c *ResourceCatalog) { c.OptionSets[0].Active = false }},
		{"option", func(c *ResourceCatalog) { c.Options[0].Active = false }},
		{"rule", func(c *ResourceCatalog) { c.Attributes[3].Rules[0].Active = false }},
		{"relation", func(c *ResourceCatalog) { c.Relations[0].Active = false }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			catalog := SeedResourceCatalog()
			tt.mutate(&catalog)
			scope, unit := conductoresScope, "M"
			values := []ResourceAttributeValue{
				OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"),
				OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V"),
			}
			if tt.name == "relation" {
				scope, unit = canalizacionesScope, "PZA"
				values = []ResourceAttributeValue{OptionValue("tipo", "CONDUIT PARED DELGADA"), OptionValue("diameter_inch", `3/4"`), OptionValue("diameter_mm", "19 mm")}
			}
			_, err := NewResource(catalog, scope, unit, values)
			if !errors.Is(err, ErrResourceReference) {
				t.Fatalf("NewResource() error = %v, want ErrResourceReference", err)
			}
		})
	}
}

func TestRehydrateResourcePreservesHistoryAfterCatalogDeactivation(t *testing.T) {
	catalog := SeedResourceCatalog()
	resource, err := NewResource(catalog, conductoresScope, "M", []ResourceAttributeValue{
		OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"),
		OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V"),
	})
	if err != nil {
		t.Fatalf("NewResource() error = %v", err)
	}
	for i := range catalog.Classes {
		catalog.Classes[i].Active = false
	}
	for i := range catalog.Families {
		catalog.Families[i].Active = false
	}
	for i := range catalog.Types {
		catalog.Types[i].Active = false
	}
	for i := range catalog.Units {
		catalog.Units[i].Active = false
	}
	got, err := RehydrateResource(catalog, ResourceSnapshot{
		ID: 42, ClassCode: resource.ClassCode, FamilyCode: resource.FamilyCode, TypeCode: resource.TypeCode,
		NaturalUnit: resource.NaturalUnit, Attributes: resource.Attributes, IdentityKey: resource.IdentityKey, Active: false,
	})
	if err != nil {
		t.Fatalf("RehydrateResource() error = %v", err)
	}
	if got.ID != 42 || got.Active {
		t.Fatalf("rehydrated historical resource = %#v, want ID 42 and inactive", got)
	}
}
