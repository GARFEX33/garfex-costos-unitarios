package domain

import (
	"strings"
	"testing"
)

// Task 2b.2: adapted from the pre-rename material_conductors_test.go.
// Every existing CONDUCTORES scenario (conditional voltage/color forbidden
// when DESNUDO, controlled-option-only values, exact-duplicate detection)
// is preserved unchanged (spec "CONDUCTORES conditional rule preserved",
// AC #19) — only the call shapes and the IdentityKey's MATERIAL| prefix
// change.

var conductoresScope = ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}

func TestConductorCatalogCreatesValidResources(t *testing.T) {
	catalog := SeedResourceCatalog()
	base := []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW-LS"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	tests := []struct {
		name   string
		unit   string
		values []ResourceAttributeValue
	}{{"valid insulated conductor", "M", base}, {"valid bare conductor", "M", []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO")}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, err := NewResource(catalog, conductoresScope, tt.unit, tt.values)
			if err != nil {
				t.Fatalf("NewResource() error = %v", err)
			}
			if resource.NaturalUnit != tt.unit {
				t.Errorf("NaturalUnit = %q, want %q", resource.NaturalUnit, tt.unit)
			}
			if resource.IdentityKey == "" {
				t.Fatal("IdentityKey must be populated")
			}
		})
	}
}

func TestConductorValidationRejectsMissingForbiddenInvalidAndUnitValues(t *testing.T) {
	catalog := SeedResourceCatalog()
	valid := []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	tests := []struct {
		name   string
		unit   string
		values []ResourceAttributeValue
	}{{"missing insulation", "M", without(valid, "insulation")}, {"missing required color", "M", without(valid, "color")}, {"missing required voltage", "M", without(valid, "voltage")}, {"bare conductor has forbidden color", "M", []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO"), OptionValue("color", "NEGRO")}}, {"bare conductor has forbidden voltage", "M", []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "DESNUDO"), OptionValue("voltage", "600 V")}}, {"invalid option", "M", replace(valid, OptionValue("gauge", "13"))}, {"invalid unit", "KG", valid}, {"empty insulation is not bare", "M", replace(valid, OptionValue("insulation", ""))}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewResource(catalog, conductoresScope, tt.unit, tt.values)
			if err == nil {
				t.Fatal("NewResource() error = nil, want validation error")
			}
		})
	}
}

func TestNaturalUnitDoesNotParticipateInIdentity(t *testing.T) {
	catalog := SeedResourceCatalog()
	values := []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	a, err := NewResource(catalog, conductoresScope, "M", values)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewResource(catalog, conductoresScope, "M", values)
	if err != nil {
		t.Fatal(err)
	}
	b.NaturalUnit = "ROLLO"
	if a.IdentityKey != b.IdentityKey {
		t.Errorf("identity keys differ: %q != %q", a.IdentityKey, b.IdentityKey)
	}
}

func TestExactDuplicateDetectionAndTechnicalDifferences(t *testing.T) {
	catalog := SeedResourceCatalog()
	makeResource := func(color, insulation, gauge string) Resource {
		r, err := NewResource(catalog, conductoresScope, "M", []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", gauge), OptionValue("insulation", insulation), OptionValue("color", color), OptionValue("voltage", "600 V")})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	base := makeResource("NEGRO", "THW-LS", "10 AWG")
	duplicate := makeResource("NEGRO", "THW-LS", "10 AWG")
	if !ExactDuplicate(base, duplicate) {
		t.Fatal("equal technical values must be an exact duplicate")
	}
	for _, different := range []Resource{makeResource("BLANCO", "THW-LS", "10 AWG"), makeResource("NEGRO", "XHHW-2", "10 AWG"), makeResource("NEGRO", "THW-LS", "12 AWG")} {
		if ExactDuplicate(base, different) {
			t.Errorf("different technical values share identity key %q", different.IdentityKey)
		}
	}
}

func TestControlledOptionsRequireOfficialCodes(t *testing.T) {
	catalog := SeedResourceCatalog()
	base := []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	for _, test := range []struct {
		name  string
		value ResourceAttributeValue
	}{{"bare gauge number is not repaired", OptionValue("gauge", "10")}, {"non-exact official spelling is rejected", OptionValue("gauge", "10 awg")}, {"free text material is rejected", OptionValue("conductor_material", "COPPER")}, {"free text insulation is rejected", OptionValue("insulation", "PVC")}, {"official gauge passes", OptionValue("gauge", "10 AWG")}} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewResource(catalog, conductoresScope, "M", replace(base, test.value))
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

// TestVoltageBehavesLikeAnyControlledOption covers voltage's
// ValueTypeControlledOption shape: an approved value builds a CABLE
// resource normally and appears correctly in IdentityKey/Attributes, while
// an unapproved value is rejected — the exact same shape already used
// above for gauge/insulation/color.
func TestVoltageBehavesLikeAnyControlledOption(t *testing.T) {
	catalog := SeedResourceCatalog()
	base := []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "10 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}

	resource, err := NewResource(catalog, conductoresScope, "M", base)
	if err != nil {
		t.Fatalf("NewResource() error = %v", err)
	}
	want := "v1|8:MATERIAL11:CONDUCTORES5:CABLE5:color17:CONTROLLED_OPTION5:NEGRO18:conductor_material17:CONTROLLED_OPTION5:COBRE5:gauge17:CONTROLLED_OPTION6:10 AWG10:insulation17:CONTROLLED_OPTION3:THW7:voltage17:CONTROLLED_OPTION5:600 V"
	if resource.IdentityKey != want {
		t.Errorf("IdentityKey = %q, want %q", resource.IdentityKey, want)
	}

	if _, err := NewResource(catalog, conductoresScope, "M", replace(base, OptionValue("voltage", "700 V"))); err == nil {
		t.Fatal("unapproved voltage option was accepted")
	}
}

// TestVoltageOptionsMatchApprovedCatalogValues covers OptionsFor(voltage)
// exposing exactly the 7 approved values, in catalog order.
func TestVoltageOptionsMatchApprovedCatalogValues(t *testing.T) {
	catalog := SeedResourceCatalog()
	voltage := findResourceAttribute(t, catalog.AttributesFor(conductoresScope), "voltage")
	want := []string{"300 V", "600 V", "1000 V", "5000 V", "15000 V", "25000 V", "35000 V"}
	options := catalog.OptionsFor(voltage)
	if len(options) != len(want) {
		t.Fatalf("OptionsFor(voltage) = %v, want %v", options, want)
	}
	for i, option := range options {
		if option.Code != want[i] {
			t.Errorf("OptionsFor(voltage)[%d].Code = %q, want %q", i, option.Code, want[i])
		}
	}
}
