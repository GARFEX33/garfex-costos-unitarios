package domain

// SeedResourceCatalog returns the approved generic catalog data (was
// NewMaterialsCatalog) — same CONDUCTORES/CANALIZACIONES seed content as
// before the recursos-maestro rename, class-scoped under ClassCode
// "MATERIAL" per design §2/D1, with every user-visible Name/Plural rendered
// in Spanish (proposal §Approach step 1, D2) and MANO_DE_OBRA /
// EQUIPO_HERRAMIENTA seeded as active classes with no family/type/attribute
// data yet (proposal In Scope #1/#2 — the platform must be right, real
// content is a later, out-of-scope phase). Only .Name/.Label/.Plural
// changed in this pass; every .Code is byte-for-byte unchanged — see
// TestKnownResourceCodesAreNeverRenderedOrTranslated (resource_codes_test.go).
// The engine below contains no family-specific branch; each family is data.
func SeedResourceCatalog() ResourceCatalog {
	definitions := catalogDefinitions()
	return ResourceCatalog{
		Classes: []ResourceClass{
			{Code: "MATERIAL", Name: "Material", Plural: "Materiales", Slug: "materiales", Order: 1, Active: true},
			{Code: "MANO_DE_OBRA", Name: "Mano de obra", Plural: "Mano de obra", Slug: "mano-de-obra", Order: 2, Active: true},
			{Code: "EQUIPO_HERRAMIENTA", Name: "Equipo/Herramienta", Plural: "Equipo/Herramienta", Slug: "equipo-herramienta", Order: 3, Active: true},
		},
		Families: []ResourceFamily{
			{ClassCode: "MATERIAL", Code: "CONDUCTORES", Name: "Conductores"},
			{ClassCode: "MATERIAL", Code: "CANALIZACIONES", Name: "Canalizaciones"},
		},
		Types: []ResourceType{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Code: "CABLE", Name: "Cable"},
			{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", Code: "TUBERIA", Name: "Tubería"},
		},
		PresentationFields: []PresentationField{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", AttributeCode: "insulation", Position: 1},
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", AttributeCode: "gauge", Position: 2},
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", AttributeCode: "color", Position: 3},
			{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA", AttributeCode: "tipo", Position: 1},
			{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA", AttributeCode: "diameter_inch", Position: 2},
		},
		Units: []UnitDefinition{
			{Code: "M", Symbol: "M", Dimension: "LENGTH"},
			{Code: "PZA", Symbol: "PZA", Dimension: "PIECE"},
		},
		UnitPolicies: []ResourceUnitPolicy{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", UnitCode: "M", Allowed: true, Suggested: true},
			{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", UnitCode: "PZA", Allowed: true, Suggested: true},
		},
		Definitions: definitions,
		Attributes: []ResourceAttribute{
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Definition: definition("conductor_material"), Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Definition: definition("gauge"), Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Definition: definition("insulation"), Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Definition: definition("color"), Mode: ModeConditional, IdentityParticipates: true, Rules: []AttributeRule{{When: AttributeCondition{AttributeCode: "insulation", Equals: "DESNUDO"}, Mode: ModeForbidden, NotApplicable: true}}},
			{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Definition: definition("voltage"), Mode: ModeConditional, IdentityParticipates: true, Rules: []AttributeRule{{When: AttributeCondition{AttributeCode: "insulation", Equals: "DESNUDO"}, Mode: ModeForbidden, NotApplicable: true}}},
			{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA", Definition: definition("tipo"), Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA", Definition: definition("diameter_inch"), Mode: ModeRequired, IdentityParticipates: true},
			{ClassCode: "MATERIAL", FamilyCode: "CANALIZACIONES", TypeCode: "TUBERIA", Definition: definition("diameter_mm"), Mode: ModeRequired, IdentityParticipates: false},
		},
		Options:   append(conductorOptions(), tuberiasOptions()...),
		Relations: tuberiasRelations(),
	}
}

func definition(code string) AttributeDefinition {
	for _, definition := range catalogDefinitions() {
		if definition.Code == code {
			return definition
		}
	}
	return AttributeDefinition{}
}

func catalogDefinitions() []AttributeDefinition {
	return []AttributeDefinition{
		{Code: "conductor_material", Name: "Material del conductor", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
		{Code: "gauge", Name: "Calibre", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
		{Code: "insulation", Name: "Aislamiento", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
		{Code: "color", Name: "Color", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
		{Code: "voltage", Name: "Voltaje", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
		{Code: "tipo", Name: "Tipo", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
		{Code: "diameter_inch", Name: "Diámetro pulgadas", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: true},
		{Code: "diameter_mm", Name: "Diámetro mm", ValueType: ValueTypeControlledOption, DefaultIdentityParticipates: false},
	}
}

func conductorOptions() []AttributeOption {
	options := []AttributeOption{}
	add := func(attribute string, values ...string) {
		for _, value := range values {
			options = append(options, AttributeOption{AttributeCode: attribute, Code: value, Label: value})
		}
	}
	add("conductor_material", "COBRE", "ALUMINIO")
	add("gauge", "14 AWG", "12 AWG", "10 AWG", "8 AWG", "6 AWG", "4 AWG", "2 AWG", "1 AWG", "1/0 AWG", "2/0 AWG", "3/0 AWG", "4/0 AWG")
	add("insulation", "DESNUDO", "THW", "THW-LS", "THHN", "THHN/THWN-2", "XHHW-2", "RHH/RHW-2")
	add("color", "NEGRO", "BLANCO", "ROJO", "AZUL", "VERDE")
	add("voltage", "300 V", "600 V", "1000 V", "5000 V", "15000 V", "25000 V", "35000 V")
	return options
}

func tuberiasOptions() []AttributeOption {
	options := []AttributeOption{}
	add := func(attribute string, values ...string) {
		for _, value := range values {
			options = append(options, AttributeOption{AttributeCode: attribute, Code: value, Label: value})
		}
	}
	add("tipo", "CONDUIT PARED DELGADA", "CONDUIT PARED GRUESA", "PVC CONDUIT")
	add("diameter_inch", `1/2"`, `3/4"`, `1"`, `1 1/4"`, `1 1/2"`, `2"`, `2 1/2"`, `3"`, `4"`)
	add("diameter_mm", "13 mm", "19 mm", "25 mm", "32 mm", "38 mm", "50 mm", "60 mm", "75 mm", "100 mm")
	return options
}

func tuberiasRelations() []AttributeOptionRelation {
	relations := []AttributeOptionRelation{}
	pairs := [][2]string{
		{`1/2"`, "13 mm"}, {`3/4"`, "19 mm"}, {`1"`, "25 mm"}, {`1 1/4"`, "32 mm"}, {`1 1/2"`, "38 mm"},
		{`2"`, "50 mm"}, {`2 1/2"`, "60 mm"}, {`3"`, "75 mm"}, {`4"`, "100 mm"},
	}
	for _, pair := range pairs {
		relations = append(relations, AttributeOptionRelation{FromAttribute: "diameter_inch", FromOption: pair[0], ToAttribute: "diameter_mm", ToOption: pair[1]})
	}
	return relations
}
