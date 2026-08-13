package domain

import "testing"

func TestTuberiasCatalogCreatesValidMaterials(t *testing.T) {
	catalog := NewMaterialsCatalog()
	tests := []struct {
		name, tipo, diameterInch, diameterMM, wantIdentity string
	}{{"conduit pared delgada 1/2", "CONDUIT PARED DELGADA", `1/2"`, "13 mm", `CANALIZACIONES|TUBERIA|diameter_inch=1/2"|tipo=CONDUIT PARED DELGADA`}, {"conduit pared gruesa 3/4", "CONDUIT PARED GRUESA", `3/4"`, "19 mm", `CANALIZACIONES|TUBERIA|diameter_inch=3/4"|tipo=CONDUIT PARED GRUESA`}, {"pvc conduit 1", "PVC CONDUIT", `1"`, "25 mm", `CANALIZACIONES|TUBERIA|diameter_inch=1"|tipo=PVC CONDUIT`}, {"conduit pared delgada 4", "CONDUIT PARED DELGADA", `4"`, "100 mm", `CANALIZACIONES|TUBERIA|diameter_inch=4"|tipo=CONDUIT PARED DELGADA`}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			material, err := NewMaterial(catalog, "CANALIZACIONES", "TUBERIA", "PZA", []MaterialAttributeValue{OptionValue("tipo", tt.tipo), OptionValue("diameter_inch", tt.diameterInch), OptionValue("diameter_mm", tt.diameterMM)})
			if err != nil {
				t.Fatalf("NewMaterial() error = %v", err)
			}
			if material.NaturalUnit != "PZA" {
				t.Errorf("NaturalUnit = %q, want PZA", material.NaturalUnit)
			}
			if material.IdentityKey != tt.wantIdentity {
				t.Errorf("IdentityKey = %q, want %q", material.IdentityKey, tt.wantIdentity)
			}
		})
	}
}

func TestTuberiasValidationRejectsInvalidTypeDiameterAndPair(t *testing.T) {
	catalog := NewMaterialsCatalog()
	valid := []MaterialAttributeValue{OptionValue("tipo", "CONDUIT PARED DELGADA"), OptionValue("diameter_inch", `1/2"`), OptionValue("diameter_mm", "13 mm")}
	tests := []struct {
		name   string
		unit   string
		values []MaterialAttributeValue
	}{{"missing tipo", "PZA", without(valid, "tipo")}, {"missing diameter_inch", "PZA", without(valid, "diameter_inch")}, {"missing diameter_mm", "PZA", without(valid, "diameter_mm")}, {"invalid tipo option", "PZA", replace(valid, OptionValue("tipo", "EMT"))}, {"invalid diameter_inch option", "PZA", replace(valid, OptionValue("diameter_inch", `1/8"`))}, {"invalid diameter_mm option", "PZA", replace(valid, OptionValue("diameter_mm", "10 mm"))}, {"incoherent inch/mm pair", "PZA", replace(valid, OptionValue("diameter_mm", "25 mm"))}, {"invalid natural unit", "M", valid}, {"invalid natural unit KG", "KG", valid}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMaterial(catalog, "CANALIZACIONES", "TUBERIA", tt.unit, tt.values)
			if err == nil {
				t.Fatal("NewMaterial() error = nil, want validation error")
			}
		})
	}
}

func TestTuberiasNaturalUnitDoesNotParticipateInIdentity(t *testing.T) {
	catalog := NewMaterialsCatalog()
	values := []MaterialAttributeValue{OptionValue("tipo", "CONDUIT PARED DELGADA"), OptionValue("diameter_inch", `1/2"`), OptionValue("diameter_mm", "13 mm")}
	a, err := NewMaterial(catalog, "CANALIZACIONES", "TUBERIA", "PZA", values)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewMaterial(catalog, "CANALIZACIONES", "TUBERIA", "PZA", values)
	if err != nil {
		t.Fatal(err)
	}
	b.NaturalUnit = "ROLLO"
	if a.IdentityKey != b.IdentityKey {
		t.Errorf("identity keys differ: %q != %q", a.IdentityKey, b.IdentityKey)
	}
}

func TestTuberiasDuplicateIdentity(t *testing.T) {
	catalog := NewMaterialsCatalog()
	values := []MaterialAttributeValue{OptionValue("tipo", "CONDUIT PARED DELGADA"), OptionValue("diameter_inch", `1/2"`), OptionValue("diameter_mm", "13 mm")}
	a, err := NewMaterial(catalog, "CANALIZACIONES", "TUBERIA", "PZA", values)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewMaterial(catalog, "CANALIZACIONES", "TUBERIA", "PZA", values)
	if err != nil {
		t.Fatal(err)
	}
	if !ExactDuplicate(a, b) {
		t.Errorf("identical materials must be exact duplicates: %q != %q", a.IdentityKey, b.IdentityKey)
	}
}

func TestTuberiasDistinctIdentitiesForTechnicalDifferences(t *testing.T) {
	catalog := NewMaterialsCatalog()
	makeMaterial := func(tipo, diameterInch, diameterMM string) Material {
		m, err := NewMaterial(catalog, "CANALIZACIONES", "TUBERIA", "PZA", []MaterialAttributeValue{OptionValue("tipo", tipo), OptionValue("diameter_inch", diameterInch), OptionValue("diameter_mm", diameterMM)})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	base := makeMaterial("CONDUIT PARED DELGADA", `1/2"`, "13 mm")
	distinct := []Material{makeMaterial("PVC CONDUIT", `1/2"`, "13 mm"), makeMaterial("CONDUIT PARED DELGADA", `3/4"`, "19 mm")}
	for _, other := range distinct {
		if ExactDuplicate(base, other) {
			t.Errorf("expected distinct identities but matched: %q", other.IdentityKey)
		}
	}
}

func TestTuberiasOldAttributesAndOptionsRemoved(t *testing.T) {
	catalog := NewMaterialsCatalog()
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

func TestTuberiasOnlyApprovedAttributesAndOptions(t *testing.T) {
	catalog := NewMaterialsCatalog()
	approvedAttributes := map[string]bool{"tipo": true, "diameter_inch": true, "diameter_mm": true}
	for _, fa := range catalog.FamilyAttributes {
		if fa.FamilyCode != "CANALIZACIONES" {
			continue
		}
		if !approvedAttributes[fa.Definition.Code] {
			t.Fatalf("unapproved attribute %q for TUBERIAS", fa.Definition.Code)
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
