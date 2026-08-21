package domain

// EquivalentResourceCatalogs reports whether a and b are the same persisted
// authority state under only the two approved normalizations (design
// "Catalog transaction and coherent publication"): default option-set
// spelling ("" == "DEFAULT") and nil vs. non-nil-empty collections of equal
// content. Everything else is UNEQUAL: omission, Active, any natural-code
// identity/reference field, and APLICABILIDAD Rules order/content.
// Comparison is strictly positional per kind. Pure and copy-safe: read-only,
// retains no reference into either input's backing storage.
func EquivalentResourceCatalogs(a, b ResourceCatalog) bool {
	return equivalentSlices(a.Classes, b.Classes, equivalentClass) &&
		equivalentSlices(a.Families, b.Families, equivalentFamily) &&
		equivalentSlices(a.Types, b.Types, equivalentType) &&
		equivalentSlices(a.PresentationFields, b.PresentationFields, equivalentPresentationField) &&
		equivalentSlices(a.Units, b.Units, equivalentUnit) &&
		equivalentSlices(a.UnitPolicies, b.UnitPolicies, equivalentUnitPolicy) &&
		equivalentSlices(a.Definitions, b.Definitions, equivalentDefinition) &&
		equivalentSlices(a.Attributes, b.Attributes, equivalentAttribute) &&
		equivalentSlices(a.Options, b.Options, equivalentOption) &&
		equivalentSlices(a.Relations, b.Relations, equivalentRelation) &&
		equivalentSlices(a.OptionSets, b.OptionSets, equivalentOptionSet)
}

// normalizedOptionSet is the sole approved spelling normalization: "" and "DEFAULT" name the same set.
func normalizedOptionSet(code string) string {
	if code == "" {
		return defaultOptionSet
	}
	return code
}

// equivalentSlices: cardinality (omission) and positional content must both match.
func equivalentSlices[T any](a, b []T, equal func(T, T) bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if !equal(v, b[i]) {
			return false
		}
	}
	return true
}

// stringsEqual: equal length plus per-index equality treats nil and
// non-nil-empty slices as equal without extra special-casing.
func stringsEqual(a, b []string) bool {
	return equivalentSlices(a, b, func(x, y string) bool { return x == y })
}

func equivalentClass(a, b ResourceClass) bool {
	return a.Code == b.Code && a.Name == b.Name && a.Plural == b.Plural && a.Slug == b.Slug &&
		a.Order == b.Order && a.Active == b.Active &&
		stringsEqual(a.Aliases, b.Aliases) && stringsEqual(a.Keywords, b.Keywords)
}

func equivalentFamily(a, b ResourceFamily) bool {
	return a.ClassCode == b.ClassCode && a.Code == b.Code && a.Name == b.Name && a.Active == b.Active
}

func equivalentType(a, b ResourceType) bool {
	return a.ClassCode == b.ClassCode && a.FamilyCode == b.FamilyCode && a.Code == b.Code &&
		a.Name == b.Name && a.Active == b.Active
}

func equivalentDefinition(a, b AttributeDefinition) bool {
	return a.Code == b.Code && a.Name == b.Name && a.ValueType == b.ValueType && a.Dimension == b.Dimension &&
		a.DefaultIdentityParticipates == b.DefaultIdentityParticipates && a.Active == b.Active
}

func equivalentOptionSet(a, b ResourceOptionSet) bool {
	return normalizedOptionSet(a.Code) == normalizedOptionSet(b.Code) && a.Name == b.Name && a.Active == b.Active
}

func equivalentOption(a, b AttributeOption) bool {
	return normalizedOptionSet(a.OptionSet) == normalizedOptionSet(b.OptionSet) &&
		a.AttributeCode == b.AttributeCode && a.Code == b.Code && a.Label == b.Label && a.Active == b.Active
}

func equivalentRelation(a, b AttributeOptionRelation) bool {
	return normalizedOptionSet(a.OptionSet) == normalizedOptionSet(b.OptionSet) &&
		a.FromAttribute == b.FromAttribute && a.FromOption == b.FromOption &&
		a.ToAttribute == b.ToAttribute && a.ToOption == b.ToOption && a.Active == b.Active
}

func equivalentUnit(a, b UnitDefinition) bool {
	return a.Code == b.Code && a.Name == b.Name && a.Symbol == b.Symbol && a.Dimension == b.Dimension && a.Active == b.Active
}

func equivalentUnitPolicy(a, b ResourceUnitPolicy) bool {
	return a.ClassCode == b.ClassCode && a.FamilyCode == b.FamilyCode && a.UnitCode == b.UnitCode &&
		a.Allowed == b.Allowed && a.Suggested == b.Suggested && a.Active == b.Active
}

// equivalentAttribute: APLICABILIDAD's Rules compare positionally, so a
// reordered rule list with the same elements is unequal (design 4A). The
// embedded materialized Definition compares by every field (reusing
// equivalentDefinition), not just Code, so a stale materialized definition
// (same Code, different Name/ValueType/etc.) is correctly UNEQUAL.
func equivalentAttribute(a, b ResourceAttribute) bool {
	return a.ClassCode == b.ClassCode && a.FamilyCode == b.FamilyCode && a.TypeCode == b.TypeCode &&
		normalizedOptionSet(a.OptionSet) == normalizedOptionSet(b.OptionSet) &&
		equivalentDefinition(a.Definition, b.Definition) && a.Mode == b.Mode &&
		a.IdentityParticipates == b.IdentityParticipates && a.Active == b.Active &&
		equivalentSlices(a.Rules, b.Rules, equivalentRule)
}

func equivalentRule(a, b AttributeRule) bool {
	return a.When == b.When && a.Mode == b.Mode && a.IdentityParticipates == b.IdentityParticipates &&
		a.NotApplicable == b.NotApplicable && a.Active == b.Active
}

func equivalentPresentationField(a, b PresentationField) bool {
	return a.ClassCode == b.ClassCode && a.FamilyCode == b.FamilyCode && a.TypeCode == b.TypeCode &&
		a.AttributeCode == b.AttributeCode && a.Position == b.Position && a.Active == b.Active
}
