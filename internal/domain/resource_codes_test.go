package domain

import "testing"

// TestKnownResourceCodesAreNeverRenderedOrTranslated is the recursos-maestro
// PR3 guard rail (spec "Known codes never rendered or translated", proposal
// D2): the Spanish rename pass (3.2) touches only .Name/.Label/.Plural,
// never .Code. This frozen list must stay byte-for-byte stable across the
// rename — if any of these assertions fail, a Code was accidentally
// translated instead of the field that should have been.
func TestKnownResourceCodesAreNeverRenderedOrTranslated(t *testing.T) {
	catalog := NewResourceCatalog()

	t.Run("class codes", func(t *testing.T) {
		wantCodes := map[string]bool{"MATERIAL": true, "MANO_DE_OBRA": true, "EQUIPO_HERRAMIENTA": true}
		gotCodes := map[string]bool{}
		for _, class := range catalog.Classes {
			gotCodes[class.Code] = true
		}
		for code := range wantCodes {
			if !gotCodes[code] {
				t.Errorf("expected class code %q to be seeded, got classes %+v", code, catalog.Classes)
			}
		}
	})

	t.Run("family codes", func(t *testing.T) {
		wantCodes := map[string]bool{"CONDUCTORES": true, "CANALIZACIONES": true}
		gotCodes := map[string]bool{}
		for _, family := range catalog.Families {
			gotCodes[family.Code] = true
		}
		for code := range wantCodes {
			if !gotCodes[code] {
				t.Errorf("expected family code %q to be present, got families %+v", code, catalog.Families)
			}
		}
	})

	t.Run("type codes", func(t *testing.T) {
		wantCodes := map[string]bool{"CABLE": true, "TUBERIA": true}
		gotCodes := map[string]bool{}
		for _, resourceType := range catalog.Types {
			gotCodes[resourceType.Code] = true
		}
		for code := range wantCodes {
			if !gotCodes[code] {
				t.Errorf("expected type code %q to be present, got types %+v", code, catalog.Types)
			}
		}
	})

	t.Run("attribute definition codes", func(t *testing.T) {
		wantCodes := map[string]bool{
			"conductor_material": true,
			"gauge":              true,
			"insulation":         true,
			"color":              true,
			"voltage":            true,
			"tipo":               true,
			"diameter_inch":      true,
			"diameter_mm":        true,
		}
		gotCodes := map[string]bool{}
		for _, definition := range catalog.Definitions {
			gotCodes[definition.Code] = true
		}
		for code := range wantCodes {
			if !gotCodes[code] {
				t.Errorf("expected attribute definition code %q to be present, got definitions %+v", code, catalog.Definitions)
			}
		}
	})
}

// TestEveryRenderedCatalogNameIsNonEmpty is a cheap sanity check against
// typos during the mechanical Spanish rewrite (3.2): every user-visible
// Name/Label/Plural in the production catalog must be non-empty.
func TestEveryRenderedCatalogNameIsNonEmpty(t *testing.T) {
	catalog := NewResourceCatalog()

	for _, class := range catalog.Classes {
		if class.Name == "" {
			t.Errorf("class %q has empty Name", class.Code)
		}
		if class.Plural == "" {
			t.Errorf("class %q has empty Plural", class.Code)
		}
	}

	for _, family := range catalog.Families {
		if family.Name == "" {
			t.Errorf("family %q (class %q) has empty Name", family.Code, family.ClassCode)
		}
	}

	for _, resourceType := range catalog.Types {
		if resourceType.Name == "" {
			t.Errorf("type %q (family %q) has empty Name", resourceType.Code, resourceType.FamilyCode)
		}
	}

	for _, definition := range catalog.Definitions {
		if definition.Name == "" {
			t.Errorf("attribute definition %q has empty Name", definition.Code)
		}
	}

	for _, option := range catalog.Options {
		if option.Label == "" {
			t.Errorf("option %q (attribute %q) has empty Label", option.Code, option.AttributeCode)
		}
	}
}

// TestSeededCatalogValidatesCleanWithNewClasses proves 3.3's assumption
// rather than assuming it: MANO_DE_OBRA/EQUIPO_HERRAMIENTA are seeded with
// zero families/types/attributes, and ResourceCatalog.Validate() (PR1)
// walks Families/Types/Attributes/UnitPolicies looking for dangling
// references — a class with no rows underneath it simply contributes no
// entries to check, so Validate() must still return nil for the real,
// three-class production catalog.
func TestSeededCatalogValidatesCleanWithNewClasses(t *testing.T) {
	catalog := NewResourceCatalog()
	if len(catalog.Classes) != 3 {
		t.Fatalf("expected 3 seeded classes, got %d: %+v", len(catalog.Classes), catalog.Classes)
	}
	if err := catalog.Validate(); err != nil {
		t.Fatalf("expected NewResourceCatalog().Validate() == nil, got %v", err)
	}
}
