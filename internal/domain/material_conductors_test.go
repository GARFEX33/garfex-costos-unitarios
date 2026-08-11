package domain

import (
	"strings"
	"testing"
)

func TestConductorCatalogCreatesValidMaterials(t *testing.T) {
	catalog := NewMaterialsCatalog()
	base := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW-LS"), OptionValue("color", "NEGRO"), QuantityValue("voltage", "600", "V")}
	tests := []struct {
		name   string
		unit   string
		values []MaterialAttributeValue
	}{{"valid insulated conductor", "M", base}, {"valid bare conductor", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO")}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			material, err := NewMaterial(catalog, "CONDUCTORES", tt.unit, tt.values)
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
	valid := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), QuantityValue("voltage", "600", "V")}
	tests := []struct {
		name   string
		unit   string
		values []MaterialAttributeValue
	}{{"missing insulation", "M", without(valid, "insulation")}, {"missing required color", "M", without(valid, "color")}, {"missing required voltage", "M", without(valid, "voltage")}, {"bare conductor has forbidden color", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO"), OptionValue("color", "NEGRO")}}, {"bare conductor has forbidden voltage", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO"), QuantityValue("voltage", "600", "V")}}, {"invalid option", "M", replace(valid, OptionValue("gauge", "13"))}, {"invalid unit", "KG", valid}, {"empty insulation is not bare", "M", replace(valid, OptionValue("insulation", ""))}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMaterial(catalog, "CONDUCTORES", tt.unit, tt.values)
			if err == nil {
				t.Fatal("NewMaterial() error = nil, want validation error")
			}
		})
	}
}

func TestNaturalUnitDoesNotParticipateInIdentity(t *testing.T) {
	catalog := NewMaterialsCatalog()
	values := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), QuantityValue("voltage", "600", "V")}
	a, err := NewMaterial(catalog, "CONDUCTORES", "M", values)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewMaterial(catalog, "CONDUCTORES", "M", values)
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
		m, err := NewMaterial(catalog, "CONDUCTORES", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", gauge), OptionValue("insulation", insulation), OptionValue("color", color), QuantityValue("voltage", "600", "V")})
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
	base := []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), QuantityValue("voltage", "600", "V")}
	for _, test := range []struct {
		name  string
		value MaterialAttributeValue
	}{{"bare gauge number is not repaired", OptionValue("gauge", "10")}, {"non-exact official spelling is rejected", OptionValue("gauge", "10 awg")}, {"free text material is rejected", OptionValue("conductor_material", "COPPER")}, {"free text insulation is rejected", OptionValue("insulation", "PVC")}, {"official gauge passes", OptionValue("gauge", "10 AWG")}} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewMaterial(catalog, "CONDUCTORES", "M", replace(base, test.value))
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

func TestVoltageIdentityUsesNormalizedDimension(t *testing.T) {
	catalog := NewMaterialsCatalog()
	makeMaterial := func(value, unit string) Material {
		material, err := NewMaterial(catalog, "CONDUCTORES", "M", []MaterialAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), QuantityValue("voltage", value, unit)})
		if err != nil {
			t.Fatal(err)
		}
		return material
	}
	for _, test := range []struct{ value, unit, want string }{{"1", "kV", "CONDUCTORES|color=NEGRO|conductor_material=COBRE|gauge=10 AWG|insulation=THW|voltage=1000 V"}, {"1000", "V", "CONDUCTORES|color=NEGRO|conductor_material=COBRE|gauge=10 AWG|insulation=THW|voltage=1000 V"}, {"5", "kV", "CONDUCTORES|color=NEGRO|conductor_material=COBRE|gauge=10 AWG|insulation=THW|voltage=5000 V"}, {"5000", "V", "CONDUCTORES|color=NEGRO|conductor_material=COBRE|gauge=10 AWG|insulation=THW|voltage=5000 V"}} {
		t.Run(test.value+test.unit, func(t *testing.T) {
			material := makeMaterial(test.value, test.unit)
			if material.IdentityKey != test.want {
				t.Errorf("IdentityKey = %q, want %q", material.IdentityKey, test.want)
			}
		})
	}
	if !ExactDuplicate(makeMaterial("1", "kV"), makeMaterial("1000", "V")) {
		t.Fatal("equivalent voltage quantities must be exact duplicates")
	}
}

func TestVoltageIsNotAControlledOption(t *testing.T) {
	for _, option := range NewMaterialsCatalog().Options {
		if option.AttributeCode == "voltage" {
			t.Fatalf("voltage option %q must not be present", option.Code)
		}
	}
}
