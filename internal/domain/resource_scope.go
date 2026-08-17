package domain

// ResourceScope narrows a catalog lookup by Clase, Familia, and (optionally)
// Tipo. It is the stable value passed through catalog validation and query
// boundaries.
//
// TypeCode == "" means a family-level query: no Tipo narrowing is applied.
//
// The scope is class-owned: a family or type code from another class never
// matches.
type ResourceScope struct {
	ClassCode  string
	FamilyCode string
	TypeCode   string
}

// canonicalize applies canonical() to all three fields exactly once.
func (s ResourceScope) canonicalize() ResourceScope {
	return ResourceScope{
		ClassCode:  canonical(s.ClassCode),
		FamilyCode: canonical(s.FamilyCode),
		TypeCode:   canonical(s.TypeCode),
	}
}

// matches is the single narrowing predicate every catalog query method
// shares. classCode/familyCode/typeCode are one catalog row's own scoping
// fields; s is the query scope being resolved against that row.
//
// An empty typeCode on the catalog row is a wildcard: "shared by every Tipo
// of this Familia". An empty s.TypeCode is a
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
