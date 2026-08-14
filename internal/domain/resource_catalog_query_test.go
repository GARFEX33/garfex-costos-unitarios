package domain

import "testing"

// RED (task 2a.1): ResourceScope-based lookups must reject cross-class
// family/type reuse. Both cases fail to compile until 2a.2's GREEN lands
// NewResourceCatalog()/hasFamily(ResourceScope)/hasType(ResourceScope) —
// the mechanical-rename-slice exception to RED-via-compiler (design
// Testing Strategy). PR2b later extends this same file with the full
// adapted table-case coverage from material_catalog_query_test.go.

// Spec scenario "Family valid only within its class".
func TestResourceCatalog_hasFamily_ScopedByClass(t *testing.T) {
	catalog := NewResourceCatalog()
	if !catalog.hasFamily(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES"}) {
		t.Fatalf("hasFamily(MATERIAL/CONDUCTORES) = false, want true")
	}
	if catalog.hasFamily(ResourceScope{ClassCode: "MANO_DE_OBRA", FamilyCode: "CONDUCTORES"}) {
		t.Fatalf("hasFamily(MANO_DE_OBRA/CONDUCTORES) = true, want false — CONDUCTORES is owned by MATERIAL, not MANO_DE_OBRA")
	}
}

// Spec scenario "Type valid only within its family+class".
func TestResourceCatalog_hasType_ScopedByFamily(t *testing.T) {
	catalog := NewResourceCatalog()
	if !catalog.hasType(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE"}) {
		t.Fatalf("hasType(MATERIAL/CONDUCTORES/CABLE) = false, want true")
	}
	if catalog.hasType(ResourceScope{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "CABLE"}) {
		t.Fatalf("hasType(MATERIAL/CANALIZACIONES/CABLE) = true, want false — CABLE is owned by family CONDUCTORES, not CANALIZACIONES")
	}
}

// RED (task 2a.3): named OptionSet narrowing. Spec scenario "Shared
// attribute code, different option sets".
func TestResourceCatalog_OptionsFor_NarrowsByNamedOptionSet(t *testing.T) {
	catalog := ResourceCatalog{
		Attributes: []ResourceAttribute{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Definition: AttributeDefinition{Code: "color"}, OptionSet: "COLORES_CONDUCTOR"},
			{ClassCode: "EQUIPO_HERRAMIENTA", FamilyCode: "HERRAMIENTA_MANUAL", Definition: AttributeDefinition{Code: "color"}, OptionSet: "COLORES_EQUIPO"},
		},
		Options: []AttributeOption{
			{OptionSet: "COLORES_CONDUCTOR", AttributeCode: "color", Code: "NEGRO", Label: "Negro"},
			{OptionSet: "COLORES_EQUIPO", AttributeCode: "color", Code: "AMARILLO", Label: "Amarillo"},
		},
	}
	materialColor, equipoColor := catalog.Attributes[0], catalog.Attributes[1]

	materialOptions := catalog.OptionsFor(materialColor)
	if len(materialOptions) != 1 || materialOptions[0].Code != "NEGRO" {
		t.Fatalf("OptionsFor(material color) = %+v, want only NEGRO", materialOptions)
	}
	equipoOptions := catalog.OptionsFor(equipoColor)
	if len(equipoOptions) != 1 || equipoOptions[0].Code != "AMARILLO" {
		t.Fatalf("OptionsFor(equipo color) = %+v, want only AMARILLO", equipoOptions)
	}
}

// Spec scenario "Explicit shared option set".
func TestResourceCatalog_OptionsFor_ExplicitSharedOptionSet(t *testing.T) {
	catalog := ResourceCatalog{
		Attributes: []ResourceAttribute{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Definition: AttributeDefinition{Code: "color"}, OptionSet: "COLORES_BASICOS"},
			{ClassCode: "EQUIPO_HERRAMIENTA", FamilyCode: "HERRAMIENTA_MANUAL", Definition: AttributeDefinition{Code: "color"}, OptionSet: "COLORES_BASICOS"},
		},
		Options: []AttributeOption{
			{OptionSet: "COLORES_BASICOS", AttributeCode: "color", Code: "NEGRO", Label: "Negro"},
			{OptionSet: "COLORES_BASICOS", AttributeCode: "color", Code: "ROJO", Label: "Rojo"},
		},
	}
	a := catalog.OptionsFor(catalog.Attributes[0])
	b := catalog.OptionsFor(catalog.Attributes[1])
	if len(a) != 2 || len(b) != 2 || a[0].Code != b[0].Code || a[1].Code != b[1].Code {
		t.Fatalf("OptionsFor for two classes explicitly sharing COLORES_BASICOS = %+v vs %+v, want identical", a, b)
	}
}
