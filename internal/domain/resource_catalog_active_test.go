package domain

import "testing"

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
