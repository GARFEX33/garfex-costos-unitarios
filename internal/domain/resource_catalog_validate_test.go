package domain

import (
	"strings"
	"testing"
)

// validResourceCatalog is the well-formed baseline every defect-case test
// mutates exactly one way. It deliberately reuses the family code
// CONDUCTORES under two different classes to keep the D1 invariant
// ("family codes are unique within a class, not globally") exercised by
// every run, not just its own dedicated test.
func validResourceCatalog() ResourceCatalog {
	return ResourceCatalog{
		Classes: []ResourceClass{
			{Code: "MATERIAL", Name: "Material", Plural: "Materiales", Slug: "materiales", Order: 1, Active: true},
			{Code: "MANO_DE_OBRA", Name: "Mano de obra", Plural: "Mano de obra", Slug: "mano-de-obra", Order: 2, Active: true},
		},
		Families: []ResourceFamily{
			{ClassCode: "MATERIAL", Code: "CONDUCTORES", Name: "Conductores"},
			{ClassCode: "MANO_DE_OBRA", Code: "CONDUCTORES", Name: "Conductores"},
		},
		Types: []ResourceType{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Code: "CABLE", Name: "Cable"},
		},
		Attributes: []ResourceAttribute{
			{
				ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE",
				OptionSet:  "COLORES",
				Definition: AttributeDefinition{Code: "color", Name: "Color", ValueType: ValueTypeControlledOption},
			},
		},
		Options: []AttributeOption{
			{OptionSet: "COLORES", AttributeCode: "color", Code: "NEGRO", Label: "Negro"},
		},
		Units: []UnitDefinition{
			{Code: "M", Symbol: "M", Dimension: "LENGTH"},
		},
		UnitPolicies: []ResourceUnitPolicy{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", UnitCode: "M", Allowed: true, Suggested: true},
		},
	}
}

func TestResourceCatalog_Validate_WellFormedCatalogIsClean(t *testing.T) {
	if err := validResourceCatalog().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestResourceCatalog_Validate_FamilyCodeReusedAcrossClassesIsValid(t *testing.T) {
	// The baseline already reuses CONDUCTORES across MATERIAL and
	// MANO_DE_OBRA (D1). This test names that invariant explicitly so a
	// future baseline change can't silently drop the positive case.
	catalog := validResourceCatalog()
	if err := catalog.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil (same family code under a different class is valid per D1)", err)
	}
}

func TestResourceCatalog_Validate_DefectCases(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(ResourceCatalog) ResourceCatalog
		wantErr string
	}{
		{
			name: "dangling class reference on a family",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Families = append(c.Families, ResourceFamily{ClassCode: "GHOST", Code: "X", Name: "X"})
				return c
			},
			wantErr: "unknown class",
		},
		{
			name: "dangling family reference on a type",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Types = append(c.Types, ResourceType{ClassCode: "MATERIAL", FamilyCode: "GHOST", Code: "X", Name: "X"})
				return c
			},
			wantErr: "unknown family",
		},
		{
			name: "dangling type reference on an attribute",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Attributes = append(c.Attributes, ResourceAttribute{
					ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "GHOST",
					Definition: AttributeDefinition{Code: "x"},
				})
				return c
			},
			wantErr: "unknown type",
		},
		{
			name: "dangling option-set reference on an attribute",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Attributes = append(c.Attributes, ResourceAttribute{
					ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES",
					OptionSet:  "GHOST_SET",
					Definition: AttributeDefinition{Code: "x"},
				})
				return c
			},
			wantErr: "unknown option set",
		},
		{
			name: "unknown unit in a unit policy",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.UnitPolicies = append(c.UnitPolicies, ResourceUnitPolicy{
					ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", UnitCode: "GHOST_UNIT", Allowed: true,
				})
				return c
			},
			wantErr: "unknown unit",
		},
		{
			name: "duplicate family code within one class is rejected",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Families = append(c.Families, ResourceFamily{ClassCode: "MATERIAL", Code: "CONDUCTORES", Name: "Conductores duplicado"})
				return c
			},
			wantErr: "duplicate family code",
		},
		{
			name: "duplicate class code",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Classes = append(c.Classes, ResourceClass{Code: "MATERIAL", Name: "Otro", Plural: "Otros", Slug: "otros"})
				return c
			},
			wantErr: "duplicate class code",
		},
		{
			name: "duplicate class slug",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Classes = append(c.Classes, ResourceClass{Code: "OTRO", Name: "Otro", Plural: "Otros", Slug: "materiales"})
				return c
			},
			wantErr: "duplicate class slug",
		},
		{
			name: "empty class name",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Classes = append(c.Classes, ResourceClass{Code: "OTRO", Name: "   ", Plural: "Otros", Slug: "otros"})
				return c
			},
			wantErr: "empty name",
		},
		{
			name: "empty class plural",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Classes = append(c.Classes, ResourceClass{Code: "OTRO", Name: "Otro", Plural: "", Slug: "otros"})
				return c
			},
			wantErr: "empty plural",
		},
		{
			name: "empty class slug",
			mutate: func(c ResourceCatalog) ResourceCatalog {
				c.Classes = append(c.Classes, ResourceClass{Code: "OTRO", Name: "Otro", Plural: "Otros", Slug: ""})
				return c
			},
			wantErr: "empty slug",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := tt.mutate(validResourceCatalog())
			err := catalog.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %q, want an error containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
