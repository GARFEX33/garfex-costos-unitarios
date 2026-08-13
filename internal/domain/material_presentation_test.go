package domain

import (
	"fmt"
	"testing"
)

// TestDescribeCableComposesInsulationGaugeColor covers D1/PR-D's core case:
// CABLE's PresentationFields (insulation, gauge, color, in that order) are
// composed after the ProductType name, while attributes NOT configured for
// presentation (conductor_material, voltage) are silently excluded — never
// an automatic dump of every attribute. Color keeps its catalog-canonical
// uppercase form ("BLANCO"); no casing/grammar layer is applied.
func TestDescribeCableComposesInsulationGaugeColor(t *testing.T) {
	catalog := NewMaterialsCatalog()
	material, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", []MaterialAttributeValue{
		OptionValue("conductor_material", "COBRE"),
		OptionValue("gauge", "12 AWG"),
		OptionValue("insulation", "THHN"),
		OptionValue("color", "BLANCO"),
		QuantityValue("voltage", "600", "V"),
	})
	if err != nil {
		t.Fatalf("NewMaterial() error = %v", err)
	}
	want := "Cable THHN 12 AWG BLANCO"
	if got := catalog.Describe(material); got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}
}

// TestDescribeTuberiaComposesTipoAndDiameter covers TUBERIA's
// PresentationFields (tipo, diameter_inch), using the ProductType's actual
// catalog Name ("Tubería").
func TestDescribeTuberiaComposesTipoAndDiameter(t *testing.T) {
	catalog := NewMaterialsCatalog()
	material, err := NewMaterial(catalog, "CANALIZACIONES", "TUBERIA", "PZA", []MaterialAttributeValue{
		OptionValue("tipo", "CONDUIT PARED DELGADA"),
		OptionValue("diameter_inch", `1/2"`),
		OptionValue("diameter_mm", "13 mm"),
	})
	if err != nil {
		t.Fatalf("NewMaterial() error = %v", err)
	}
	want := `Tubería CONDUIT PARED DELGADA 1/2"`
	if got := catalog.Describe(material); got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}
}

// TestDescribeSkipsFieldMissingEntirely covers a PresentationField whose
// attribute is simply absent from Attributes (e.g. a DESNUDO conductor,
// where color/voltage are structurally forbidden and never set) — it is
// skipped silently, no blank segment.
func TestDescribeSkipsFieldMissingEntirely(t *testing.T) {
	catalog := NewMaterialsCatalog()
	material, err := NewMaterial(catalog, "CONDUCTORES", "CABLE", "M", []MaterialAttributeValue{
		OptionValue("conductor_material", "COBRE"),
		OptionValue("gauge", "12 AWG"),
		OptionValue("insulation", "DESNUDO"),
	})
	if err != nil {
		t.Fatalf("NewMaterial() error = %v", err)
	}
	want := "Cable DESNUDO 12 AWG"
	if got := catalog.Describe(material); got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}
}

// TestDescribeSkipsFieldMarkedNotApplicable covers a PresentationField whose
// attribute IS present but carries the NotApplicableText sentinel (as a real
// Material fetched from the repository would for a DESNUDO conductor's
// color) — also skipped silently, no blank segment, no error.
func TestDescribeSkipsFieldMarkedNotApplicable(t *testing.T) {
	catalog := NewMaterialsCatalog()
	material := Material{
		FamilyCode:      "CONDUCTORES",
		ProductTypeCode: "CABLE",
		NaturalUnit:     "M",
		Attributes: []MaterialAttributeValue{
			OptionValue("conductor_material", "COBRE"),
			OptionValue("gauge", "12 AWG"),
			OptionValue("insulation", "DESNUDO"),
			{AttributeCode: "color", Type: ValueTypeControlledOption, Text: NotApplicableText},
		},
	}
	want := "Cable DESNUDO 12 AWG"
	if got := catalog.Describe(material); got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}
}

// TestDescribeWithNoPresentationFieldsReturnsOnlyName covers D1's explicit
// safe degradation: a ProductType with zero matching PresentationFields
// entries returns exactly its own Name, never a fallback to an improvised
// technical dump.
func TestDescribeWithNoPresentationFieldsReturnsOnlyName(t *testing.T) {
	catalog := MaterialsCatalog{
		ProductTypes: []ProductType{{FamilyCode: "WIDGETS", Code: "GADGET", Name: "Gadget"}},
	}
	material := Material{FamilyCode: "WIDGETS", ProductTypeCode: "GADGET", NaturalUnit: "PZA"}
	want := "Gadget"
	if got := catalog.Describe(material); got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}
}

// TestDescribeUnknownProductTypeReturnsEmpty covers a Material whose
// ProductTypeCode cannot be resolved in the catalog.
func TestDescribeUnknownProductTypeReturnsEmpty(t *testing.T) {
	catalog := NewMaterialsCatalog()
	material := Material{FamilyCode: "CONDUCTORES", ProductTypeCode: "NOPE", NaturalUnit: "M"}
	if got := catalog.Describe(material); got != "" {
		t.Fatalf("Describe() = %q, want %q", got, "")
	}
}

// TestPresentationFieldsReferenceApplicableFamilyAttributes is a
// catalog-self-consistency invariant: every PresentationFields entry's
// AttributeCode must correspond to a FamilyAttribute that is actually
// applicable to that entry's ProductTypeCode (a FamilyAttribute row whose
// FamilyCode matches the ProductType's Family and whose ProductTypeCode is
// either "" (shared) or exactly that ProductType's code).
func TestPresentationFieldsReferenceApplicableFamilyAttributes(t *testing.T) {
	catalog := NewMaterialsCatalog()
	for _, field := range catalog.PresentationFields {
		var family string
		found := false
		for _, productType := range catalog.ProductTypes {
			if productType.Code == field.ProductTypeCode {
				family = productType.FamilyCode
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("PresentationFields entry references unknown ProductTypeCode %q", field.ProductTypeCode)
		}
		applicable := false
		for _, fa := range catalog.FamilyAttributes {
			if fa.FamilyCode != family || fa.Definition.Code != field.AttributeCode {
				continue
			}
			if fa.ProductTypeCode == "" || fa.ProductTypeCode == field.ProductTypeCode {
				applicable = true
				break
			}
		}
		if !applicable {
			t.Fatalf("PresentationFields entry (%q, %q) has no applicable FamilyAttribute", field.ProductTypeCode, field.AttributeCode)
		}
	}
}

// TestPresentationFieldsNoDuplicateAttributePerProductType is a
// catalog-self-consistency invariant: no two PresentationFields entries
// share the same (ProductTypeCode, AttributeCode) pair.
func TestPresentationFieldsNoDuplicateAttributePerProductType(t *testing.T) {
	catalog := NewMaterialsCatalog()
	seen := map[string]bool{}
	for _, field := range catalog.PresentationFields {
		key := field.ProductTypeCode + "|" + field.AttributeCode
		if seen[key] {
			t.Fatalf("duplicate PresentationFields (ProductTypeCode, AttributeCode) pair %q", key)
		}
		seen[key] = true
	}
}

// TestPresentationFieldsNoDuplicatePositionPerProductType is a
// catalog-self-consistency invariant: no two PresentationFields entries
// share the same (ProductTypeCode, Position) pair.
func TestPresentationFieldsNoDuplicatePositionPerProductType(t *testing.T) {
	catalog := NewMaterialsCatalog()
	seen := map[string]bool{}
	for _, field := range catalog.PresentationFields {
		key := fmt.Sprintf("%s|%d", field.ProductTypeCode, field.Position)
		if seen[key] {
			t.Fatalf("duplicate PresentationFields (ProductTypeCode, Position) pair %q", key)
		}
		seen[key] = true
	}
}
