package domain

import "testing"

func TestResourceScope_canonicalize(t *testing.T) {
	tests := []struct {
		name  string
		scope ResourceScope
		want  ResourceScope
	}{
		{
			name:  "trims and upper-cases all three fields",
			scope: ResourceScope{ClassCode: " material ", FamilyCode: "conductores", TypeCode: "cable"},
			want:  ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
		},
		{
			name:  "empty TypeCode stays empty (family-level query)",
			scope: ResourceScope{ClassCode: "material", FamilyCode: "conductores", TypeCode: ""},
			want:  ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.canonicalize()
			if got != tt.want {
				t.Fatalf("canonicalize() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResourceScope_matches(t *testing.T) {
	tests := []struct {
		name                            string
		scope                           ResourceScope
		classCode, familyCode, typeCode string
		want                            bool
	}{
		{
			name:      "exact triple match, case/space-insensitive",
			scope:     ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
			classCode: "material", familyCode: "conductores", typeCode: "cable",
			want: true,
		},
		{
			name:      "class mismatch",
			scope:     ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
			classCode: "MANO_DE_OBRA", familyCode: "CONDUCTORES", typeCode: "CABLE",
			want: false,
		},
		{
			name:      "family mismatch",
			scope:     ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
			classCode: "MATERIAL", familyCode: "CANALIZACIONES", typeCode: "CABLE",
			want: false,
		},
		{
			name:      "type mismatch when both scope and row specify a type",
			scope:     ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
			classCode: "MATERIAL", familyCode: "CONDUCTORES", typeCode: "OTRO",
			want: false,
		},
		{
			name:      "wildcard catalog row (empty row typeCode) matches any scope type",
			scope:     ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"},
			classCode: "MATERIAL", familyCode: "CONDUCTORES", typeCode: "",
			want: true,
		},
		{
			name:      "family-level scope (empty scope TypeCode) matches a specific-type row",
			scope:     ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: ""},
			classCode: "MATERIAL", familyCode: "CONDUCTORES", typeCode: "CABLE",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.matches(tt.classCode, tt.familyCode, tt.typeCode)
			if got != tt.want {
				t.Fatalf("matches(%q,%q,%q) = %v, want %v", tt.classCode, tt.familyCode, tt.typeCode, got, tt.want)
			}
		})
	}
}
