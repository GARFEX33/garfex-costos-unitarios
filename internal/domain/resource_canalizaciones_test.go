package domain

import "testing"

// Task 2b.2: adapted from the pre-rename material_tuberias_test.go. Every
// existing CANALIZACIONES scenario (diameter_inch/diameter_mm mutual
// narrowing, invalid-pair rejection) is preserved unchanged (spec
// "CANALIZACIONES diameter narrowing preserved", AC #19) — only the call
// shapes and the IdentityKey's MATERIAL| prefix change.

var canalizacionesScope = ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA"}

func TestCanalizacionesCatalogCreatesValidResources(t *testing.T) {
	catalog := NewResourceCatalog()
	tests := []struct {
		name, tipo, diameterInch, diameterMM, wantIdentity string
	}{{"conduit pared delgada 1/2", "CONDUIT PARED DELGADA", `1/2"`, "13 mm", `MATERIAL|CANALIZACIONES|TUBERIA|diameter_inch=1/2"|tipo=CONDUIT PARED DELGADA`}, {"conduit pared gruesa 3/4", "CONDUIT PARED GRUESA", `3/4"`, "19 mm", `MATERIAL|CANALIZACIONES|TUBERIA|diameter_inch=3/4"|tipo=CONDUIT PARED GRUESA`}, {"pvc conduit 1", "PVC CONDUIT", `1"`, "25 mm", `MATERIAL|CANALIZACIONES|TUBERIA|diameter_inch=1"|tipo=PVC CONDUIT`}, {"conduit pared delgada 4", "CONDUIT PARED DELGADA", `4"`, "100 mm", `MATERIAL|CANALIZACIONES|TUBERIA|diameter_inch=4"|tipo=CONDUIT PARED DELGADA`}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, err := NewResource(catalog, canalizacionesScope, "PZA", []ResourceAttributeValue{OptionValue("tipo", tt.tipo), OptionValue("diameter_inch", tt.diameterInch), OptionValue("diameter_mm", tt.diameterMM)})
			if err != nil {
				t.Fatalf("NewResource() error = %v", err)
			}
			if resource.NaturalUnit != "PZA" {
				t.Errorf("NaturalUnit = %q, want PZA", resource.NaturalUnit)
			}
			if resource.IdentityKey != tt.wantIdentity {
				t.Errorf("IdentityKey = %q, want %q", resource.IdentityKey, tt.wantIdentity)
			}
		})
	}
}

func TestCanalizacionesValidationRejectsInvalidTypeDiameterAndPair(t *testing.T) {
	catalog := NewResourceCatalog()
	valid := []ResourceAttributeValue{OptionValue("tipo", "CONDUIT PARED DELGADA"), OptionValue("diameter_inch", `1/2"`), OptionValue("diameter_mm", "13 mm")}
	tests := []struct {
		name   string
		unit   string
		values []ResourceAttributeValue
	}{{"missing tipo", "PZA", without(valid, "tipo")}, {"missing diameter_inch", "PZA", without(valid, "diameter_inch")}, {"missing diameter_mm", "PZA", without(valid, "diameter_mm")}, {"invalid tipo option", "PZA", replace(valid, OptionValue("tipo", "EMT"))}, {"invalid diameter_inch option", "PZA", replace(valid, OptionValue("diameter_inch", `1/8"`))}, {"invalid diameter_mm option", "PZA", replace(valid, OptionValue("diameter_mm", "10 mm"))}, {"incoherent inch/mm pair", "PZA", replace(valid, OptionValue("diameter_mm", "25 mm"))}, {"invalid natural unit", "M", valid}, {"invalid natural unit KG", "KG", valid}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewResource(catalog, canalizacionesScope, tt.unit, tt.values)
			if err == nil {
				t.Fatal("NewResource() error = nil, want validation error")
			}
		})
	}
}

func TestCanalizacionesNaturalUnitDoesNotParticipateInIdentity(t *testing.T) {
	catalog := NewResourceCatalog()
	values := []ResourceAttributeValue{OptionValue("tipo", "CONDUIT PARED DELGADA"), OptionValue("diameter_inch", `1/2"`), OptionValue("diameter_mm", "13 mm")}
	a, err := NewResource(catalog, canalizacionesScope, "PZA", values)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewResource(catalog, canalizacionesScope, "PZA", values)
	if err != nil {
		t.Fatal(err)
	}
	b.NaturalUnit = "ROLLO"
	if a.IdentityKey != b.IdentityKey {
		t.Errorf("identity keys differ: %q != %q", a.IdentityKey, b.IdentityKey)
	}
}

func TestCanalizacionesDuplicateIdentity(t *testing.T) {
	catalog := NewResourceCatalog()
	values := []ResourceAttributeValue{OptionValue("tipo", "CONDUIT PARED DELGADA"), OptionValue("diameter_inch", `1/2"`), OptionValue("diameter_mm", "13 mm")}
	a, err := NewResource(catalog, canalizacionesScope, "PZA", values)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewResource(catalog, canalizacionesScope, "PZA", values)
	if err != nil {
		t.Fatal(err)
	}
	if !ExactDuplicate(a, b) {
		t.Errorf("identical resources must be exact duplicates: %q != %q", a.IdentityKey, b.IdentityKey)
	}
}

func TestCanalizacionesDistinctIdentitiesForTechnicalDifferences(t *testing.T) {
	catalog := NewResourceCatalog()
	makeResource := func(tipo, diameterInch, diameterMM string) Resource {
		r, err := NewResource(catalog, canalizacionesScope, "PZA", []ResourceAttributeValue{OptionValue("tipo", tipo), OptionValue("diameter_inch", diameterInch), OptionValue("diameter_mm", diameterMM)})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	base := makeResource("CONDUIT PARED DELGADA", `1/2"`, "13 mm")
	distinct := []Resource{makeResource("PVC CONDUIT", `1/2"`, "13 mm"), makeResource("CONDUIT PARED DELGADA", `3/4"`, "19 mm")}
	for _, other := range distinct {
		if ExactDuplicate(base, other) {
			t.Errorf("expected distinct identities but matched: %q", other.IdentityKey)
		}
	}
}

func TestCanalizacionesOldAttributesAndOptionsRemoved(t *testing.T) {
	catalog := NewResourceCatalog()
	for _, code := range []string{"conduit_type", "material", "nominal_diameter"} {
		for _, def := range catalog.Definitions {
			if def.Code == code {
				t.Fatalf("old attribute definition %q must be removed", code)
			}
		}
		for _, opt := range catalog.Options {
			if opt.AttributeCode == code {
				t.Fatalf("old option for attribute %q must be removed", code)
			}
		}
	}
	for _, opt := range catalog.Options {
		if opt.Code == "EMT" || opt.Code == "ACERO_GALVANIZADO" {
			t.Fatalf("old option %q must be removed", opt.Code)
		}
	}
}

func TestCanalizacionesOnlyApprovedAttributesAndOptions(t *testing.T) {
	catalog := NewResourceCatalog()
	approvedAttributes := map[string]bool{"tipo": true, "diameter_inch": true, "diameter_mm": true}
	for _, attribute := range catalog.Attributes {
		if attribute.FamilyCode != "CANALIZACIONES" {
			continue
		}
		if !approvedAttributes[attribute.Definition.Code] {
			t.Fatalf("unapproved attribute %q for CANALIZACIONES", attribute.Definition.Code)
		}
	}
	approvedInch := map[string]bool{`1/2"`: true, `3/4"`: true, `1"`: true, `1 1/4"`: true, `1 1/2"`: true, `2"`: true, `2 1/2"`: true, `3"`: true, `4"`: true}
	approvedMM := map[string]bool{"13 mm": true, "19 mm": true, "25 mm": true, "32 mm": true, "38 mm": true, "50 mm": true, "60 mm": true, "75 mm": true, "100 mm": true}
	approvedTipo := map[string]bool{"CONDUIT PARED DELGADA": true, "CONDUIT PARED GRUESA": true, "PVC CONDUIT": true}
	for _, opt := range catalog.Options {
		switch opt.AttributeCode {
		case "tipo":
			if !approvedTipo[opt.Code] {
				t.Fatalf("unapproved tipo option %q", opt.Code)
			}
		case "diameter_inch":
			if !approvedInch[opt.Code] {
				t.Fatalf("unapproved diameter_inch option %q", opt.Code)
			}
		case "diameter_mm":
			if !approvedMM[opt.Code] {
				t.Fatalf("unapproved diameter_mm option %q", opt.Code)
			}
		}
	}
}
