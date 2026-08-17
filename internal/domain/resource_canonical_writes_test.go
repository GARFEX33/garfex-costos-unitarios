package domain

import "testing"

func TestNewResourceDerivesV1IdentityFromSortedCanonicalParts(t *testing.T) {
	r, err := NewResource(SeedResourceCatalog(), conductoresScope, " M ", []ResourceAttributeValue{
		OptionValue("voltage", "600 V"), OptionValue("insulation", "THW"), OptionValue("gauge", "12 AWG"), OptionValue("color", "NEGRO"), OptionValue("conductor_material", "COBRE"),
	})
	if err != nil {
		t.Fatalf("NewResource() error = %v", err)
	}
	want := "v1|8:MATERIAL11:CONDUCTORES5:CABLE5:color17:CONTROLLED_OPTION5:NEGRO18:conductor_material17:CONTROLLED_OPTION5:COBRE5:gauge17:CONTROLLED_OPTION6:12 AWG10:insulation17:CONTROLLED_OPTION3:THW7:voltage17:CONTROLLED_OPTION5:600 V"
	if r.IdentityKey != want {
		t.Fatalf("IdentityKey = %q, want %q", r.IdentityKey, want)
	}
}
func TestIdentityComponentPreventsDelimiterCollision(t *testing.T) {
	if identityComponent("a|b") == identityComponent("a")+identityComponent("b") {
		t.Fatal("length-prefixed components collided")
	}
	if identityComponent("á") != "2:á" {
		t.Fatalf("UTF-8 component = %q, want 2:á", identityComponent("á"))
	}
}
func TestRehydrateResourceVerifiesIdentityAndKeepsID(t *testing.T) {
	attrs := []ResourceAttributeValue{OptionValue("conductor_material", "COBRE"), OptionValue("gauge", "12 AWG"), OptionValue("insulation", "THW"), OptionValue("color", "NEGRO"), OptionValue("voltage", "600 V")}
	canonical, err := NewResource(SeedResourceCatalog(), conductoresScope, "M", attrs)
	if err != nil {
		t.Fatalf("NewResource() error = %v", err)
	}
	r, err := RehydrateResource(SeedResourceCatalog(), ResourceSnapshot{ID: 42, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", NaturalUnit: "M", Attributes: attrs, IdentityKey: canonical.IdentityKey})
	if err != nil || r.ID != 42 {
		t.Fatalf("RehydrateResource() = %#v, error %v", r, err)
	}
	if _, err := RehydrateResource(SeedResourceCatalog(), ResourceSnapshot{ID: 42, ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", NaturalUnit: "M", Attributes: attrs, IdentityKey: "v1|forged"}); err == nil {
		t.Fatal("forged identity was accepted")
	}
}
