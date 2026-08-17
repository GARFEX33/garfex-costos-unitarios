package domain

import "testing"

// Task 2b.5 (NEW): proves D1's class-scoping is load-bearing for identity,
// not just a naming/cosmetic change — the class segment is present and
// first in IdentityKey, and two classes with IDENTICAL family/type/
// attribute values yield DIFFERENT identity keys. A collision here would
// silently merge distinct catalog entries across classes: a correctness
// regression, not a format detail (spec "New key format").
//
// This exercises already-correct PR2a production code (the
// scope.ClassCode+"|"+... prefix in NewResource, resource_validation.go);
// see this PR's TDD Cycle Evidence table for why RED could not be forced.

// Spec "New key format": class segment is present and first, exactly as
// design specifies (class|family|type|sorted(code=canonicalValue...)).
func TestIdentityKey_HasClassSegmentFirst(t *testing.T) {
	catalog := SeedResourceCatalog()
	resource, err := NewResource(catalog, conductoresScope, "M", []ResourceAttributeValue{
		OptionValue("conductor_material", "COBRE"),
		OptionValue("gauge", "12 AWG"),
		OptionValue("insulation", "THW"),
		OptionValue("color", "NEGRO"),
		OptionValue("voltage", "600 V"),
	})
	if err != nil {
		t.Fatalf("NewResource() error = %v", err)
	}
	want := "v1|8:MATERIAL11:CONDUCTORES5:CABLE5:color17:CONTROLLED_OPTION5:NEGRO18:conductor_material17:CONTROLLED_OPTION5:COBRE5:gauge17:CONTROLLED_OPTION6:12 AWG10:insulation17:CONTROLLED_OPTION3:THW7:voltage17:CONTROLLED_OPTION5:600 V"
	if resource.IdentityKey != want {
		t.Fatalf("IdentityKey = %q, want %q", resource.IdentityKey, want)
	}
}

func identicalAttributesTwoClassCatalog() ResourceCatalog {
	return ResourceCatalog{
		Classes: []ResourceClass{
			{Code: "CLASS_ONE", Name: "Class One", Plural: "Class Ones", Slug: "class-one", Order: 1, Active: true},
			{Code: "CLASS_TWO", Name: "Class Two", Plural: "Class Twos", Slug: "class-two", Order: 2, Active: true},
		},
		Families: []ResourceFamily{
			{ClassCode: "CLASS_ONE", Code: "SAMPLE", Name: "Sample"},
			{ClassCode: "CLASS_TWO", Code: "SAMPLE", Name: "Sample"},
		},
		Types: []ResourceType{
			{ClassCode: "CLASS_ONE", FamilyCode: "SAMPLE", Code: "ITEM", Name: "Item"},
			{ClassCode: "CLASS_TWO", FamilyCode: "SAMPLE", Code: "ITEM", Name: "Item"},
		},
		Units: []UnitDefinition{{Code: "PZA", Name: "Pieza", Symbol: "PZA", Dimension: "PIECE"}},
		UnitPolicies: []ResourceUnitPolicy{
			{ClassCode: "CLASS_ONE", FamilyCode: "SAMPLE", UnitCode: "PZA", Allowed: true, Suggested: true},
			{ClassCode: "CLASS_TWO", FamilyCode: "SAMPLE", UnitCode: "PZA", Allowed: true, Suggested: true},
		},
		Definitions: []AttributeDefinition{{Code: "tag", Name: "Tag", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true}},
		Attributes: []ResourceAttribute{
			{ClassCode: "CLASS_ONE", FamilyCode: "SAMPLE", TypeCode: "ITEM", OptionSet: "TAGS", Definition: AttributeDefinition{Code: "tag", ValueType: ValueTypeControlledOption}, Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "CLASS_TWO", FamilyCode: "SAMPLE", TypeCode: "ITEM", OptionSet: "TAGS", Definition: AttributeDefinition{Code: "tag", ValueType: ValueTypeControlledOption}, Mode: ModeRequired, IdentityParticipates: true},
		},
		Options: []AttributeOption{{OptionSet: "TAGS", AttributeCode: "tag", Code: "STANDARD", Label: "Standard"}},
	}
}

// The whole point of D1's class-scoping: two classes with IDENTICAL
// family/type/attribute values (same family code "SAMPLE", same type code
// "ITEM", same tag=STANDARD, explicitly sharing OptionSet "TAGS") must
// still yield DIFFERENT identity keys — the ONLY difference is ClassCode.
func TestIdentityKey_DiffersAcrossClassesWithIdenticalFamilyTypeAttributes(t *testing.T) {
	catalog := identicalAttributesTwoClassCatalog()
	values := []ResourceAttributeValue{OptionValue("tag", "STANDARD")}

	one, err := NewResource(catalog, ResourceScope{ClassCode: "CLASS_ONE", FamilyCode: "SAMPLE", TypeCode: "ITEM"}, "PZA", values)
	if err != nil {
		t.Fatalf("NewResource(CLASS_ONE) error = %v", err)
	}
	two, err := NewResource(catalog, ResourceScope{ClassCode: "CLASS_TWO", FamilyCode: "SAMPLE", TypeCode: "ITEM"}, "PZA", values)
	if err != nil {
		t.Fatalf("NewResource(CLASS_TWO) error = %v", err)
	}

	if one.IdentityKey == two.IdentityKey {
		t.Fatalf("identical family/type/attribute values across two classes must not collide, both got %q", one.IdentityKey)
	}
	if want := "v1|9:CLASS_ONE6:SAMPLE4:ITEM3:tag17:CONTROLLED_OPTION8:STANDARD"; one.IdentityKey != want {
		t.Fatalf("IdentityKey(CLASS_ONE) = %q, want %q", one.IdentityKey, want)
	}
	if want := "v1|9:CLASS_TWO6:SAMPLE4:ITEM3:tag17:CONTROLLED_OPTION8:STANDARD"; two.IdentityKey != want {
		t.Fatalf("IdentityKey(CLASS_TWO) = %q, want %q", two.IdentityKey, want)
	}
	if ExactDuplicate(one, two) {
		t.Fatalf("ExactDuplicate(one, two) = true, want false — different classes must never be exact duplicates")
	}
}
