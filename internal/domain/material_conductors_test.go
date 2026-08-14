package domain

import (
	"strings"
	"testing"
)

func TestConductorCatalogCreatesValidMaterials(t *testing.T) {
	catalog := NewMaterialsCatalog()
	base := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW-LS"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	tests := []struct {
		name   string
		unit   string
		values []MaterialAttributeValue
	}{{"valid insulated conductor", "M", base}, {"valid bare conductor", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO")}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			material, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", tt.unit, tt.values)
			if err != nil {
				t.Fatalf("NewMaterial() error = %v", err)
			}
			if material.NaturalUnit != tt.unit {
				t.Errorf("NaturalUnit = %q, want %q", material.NaturalUnit, tt.unit)
			}
			if material.IdentityKey == "" {
				t.Fatal("IdentityKey must be populated")
			}
		})
	}
}

func TestConductorValidationRejectsMissingForbiddenInvalidAndUnitValues(t *testing.T) {
	catalog := NewMaterialsCatalog()
	valid := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	tests := []struct {
		name   string
		unit   string
		values []MaterialAttributeValue
	}{{"missing insulation", "M", without(valid, "insulation")}, {"missing required color", "M", without(valid, "color")}, {"missing required voltage", "M", without(valid, "voltage")}, {"bare conductor has forbidden color", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO"), OptionValue("color", "NEGRO")}}, {"bare conductor has forbidden voltage", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO"), OptionValue("voltage", "600 V")}}, {"invalid option", "M", replace(valid, OptionValue("gauge", "13"))}, {"invalid unit", "KG", valid}, {"empty insulation is not bare", "M", replace(valid, OptionValue("insulation", ""))}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", tt.unit, tt.values)
			if err == nil {
				t.Fatal("NewMaterial() error = nil, want validation error")
			}
		})
	}
}

func TestNaturalUnitDoesNotParticipateInIdentity(t *testing.T) {
	catalog := NewMaterialsCatalog()
	values := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	a, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", values)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", values)
	if err != nil {
		t.Fatal(err)
	}
	b.NaturalUnit = "ROLLO"
	if a.IdentityKey != b.IdentityKey {
		t.Errorf("identity keys differ: %q != %q", a.IdentityKey, b.IdentityKey)
	}
}

func TestExactDuplicateDetectionAndTechnicalDifferences(t *testing.T) {
	catalog := NewMaterialsCatalog()
	makeMaterial := func(color, insulation, gauge string) Material {
		m, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", gauge), OptionValue("insulation", insulation), OptionValue("color", color), OptionValue("voltage", "600 V")})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	base := makeMaterial("NEGRO", "THW-LS", "10 AWG")
	duplicate := makeMaterial("NEGRO", "THW-LS", "10 AWG")
	if !ExactDuplicate(base, duplicate) {
		t.Fatal("equal technical values must be an exact duplicate")
	}
	for _, different := range []Material{makeMaterial("BLANCO", "THW-LS", "10 AWG"), makeMaterial("NEGRO", "XHHW-2", "10 AWG"), makeMaterial("NEGRO", "THW-LS", "12 AWG")} {
		if ExactDuplicate(base, different) {
			t.Errorf("different technical values share identity key %q", different.IdentityKey)
		}
	}
}

func TestControlledOptionsRequireOfficialCodes(t *testing.T) {
	catalog := NewMaterialsCatalog()
	base := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	for _, test := range []struct {
		name  string
		value MaterialAttributeValue
	}{{"bare gauge number is not repaired", OptionValue("gauge", "10")}, {"non-exact official spelling is rejected", OptionValue("gauge", "10 awg")}, {"free text material is rejected", OptionValue("conductor_material", "COPPER")}, {"free text insulation is rejected", OptionValue("insulation", "PVC")}, {"official gauge passes", OptionValue("gauge", "10 AWG")}} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", replace(base, test.value))
			if strings.HasPrefix(test.name, "official") {
				if err != nil {
					t.Fatalf("official option rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("free or repaired option was accepted")
			}
		})
	}
}

// TestVoltageBehavesLikeAnyControlledOption covers voltage's move to
// ValueTypeControlledOption (Adjustment A): an approved value builds a
// CABLE material normally and appears correctly in IdentityKey/Attributes,
// while an unapproved value is rejected — the exact same shape already used
// above for gauge/insulation/color.
func TestVoltageBehavesLikeAnyControlledOption(t *testing.T) {
	catalog := NewMaterialsCatalog()
	base := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}

	material, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", base)
	if err != nil {
		t.Fatalf("NewMaterial() error = %v", err)
	}
	want := "CONDUCTORES|CABLE|color=NEGRO|conductor_material=COBRE|gauge=10 AWG|insulation=THW|voltage=600 V"
	if material.IdentityKey != want {
		t.Errorf("IdentityKey = %q, want %q", material.IdentityKey, want)
	}

	if _, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", replace(base, OptionValue("voltage", "700 V"))); err == nil {
		t.Fatal("unapproved voltage option was accepted")
	}
}

// TestVoltageOptionsMatchApprovedCatalogValues covers OptionsFor("voltage")
// exposing exactly the 7 approved values, in catalog order.
func TestVoltageOptionsMatchApprovedCatalogValues(t *testing.T) {
	catalog := NewMaterialsCatalog()
	want := []string{"300 V", "600 V", "1000 V", "5000 V", "15000 V", "25000 V", "35000 V"}
	options := catalog.OptionsFor("voltage")
	if len(options) != len(want) {
		t.Fatalf("OptionsFor(voltage) = %v, want %v", options, want)
	}
	for i, option := range options {
		if option.Code != want[i] {
			t.Errorf("OptionsFor(voltage)[%d].Code = %q, want %q", i, option.Code, want[i])
		}
	}
}
