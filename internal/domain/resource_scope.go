package domain

// ResourceScope narrows a catalog lookup by Clase, Familia, and (optionally)
// Tipo. It replaces a third positional string parameter — a well-known Go
// footgun once you have three adjacent strings — with one stable value type,
// so a future fourth narrowing level can be added without breaking every
// call site (recursos-maestro design §1, D1).
//
// TypeCode == "" means a family-level query: no Tipo narrowing is applied.
//
// This is an additive, PR1-scoped type: nothing in the existing Materials
// model (MaterialFamily, ProductType, FamilyAttribute, ...) consumes it yet.
// Later phases of recursos-maestro thread it through the catalog query and
// validation methods that today take bare family/productType strings.
type ResourceScope struct {
	ClassCode  string
	FamilyCode string
	TypeCode   string
}

// canonicalize applies canonical() to all three fields exactly once,
// mirroring the canonical(family)/canonical(productType) prologue every
// MaterialsCatalog query method already performs today.
func (s ResourceScope) canonicalize() ResourceScope {
	return ResourceScope{
		ClassCode:  canonical(s.ClassCode),
		FamilyCode: canonical(s.FamilyCode),
		TypeCode:   canonical(s.TypeCode),
	}
}

// matches is the single narrowing predicate later catalog query methods
// share. classCode/familyCode/typeCode are one catalog row's own scoping
// fields; s is the query scope being resolved against that row.
//
// An empty typeCode on the CATALOG ROW is a wildcard: "shared by every Tipo
// of this Familia" — the exact FamilyAttribute.ProductTypeCode == ""
// semantics MaterialsCatalog already has today. An empty s.TypeCode is a
// family-level query and likewise does not constrain by Tipo (design §1:
// TypesFor/NaturalUnitsFor ignore TypeCode). Type only blocks a match when
// BOTH sides name one.
func (s ResourceScope) matches(classCode, familyCode, typeCode string) bool {
	scope := s.canonicalize()
	if scope.ClassCode != canonical(classCode) {
		return false
	}
	if scope.FamilyCode != canonical(familyCode) {
		return false
	}
	rowType := canonical(typeCode)
	if rowType == "" || scope.TypeCode == "" {
		return true
	}
	return rowType == scope.TypeCode
}
