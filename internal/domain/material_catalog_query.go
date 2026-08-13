package domain

// AttributesFor returns the FamilyAttributes applicable to family+productType
// (both catalog-declared-family-scoped and productType-specific), in the
// catalog's own declaration order — the order a UI should ask about them in.
// Unknown family/productType simply yields an empty result, not an error:
// this is a read query, not a validator (see NewMaterial for validation).
func (c MaterialsCatalog) AttributesFor(family, productType string) []FamilyAttribute {
	family = canonical(family)
	productType = canonical(productType)
	var attributes []FamilyAttribute
	for _, attribute := range c.FamilyAttributes {
		if canonical(attribute.FamilyCode) != family {
			continue
		}
		scope := canonical(attribute.ProductTypeCode)
		if scope != "" && scope != productType {
			continue
		}
		attributes = append(attributes, attribute)
	}
	return attributes
}

// OptionsFor returns every catalog-controlled option for attributeCode, in
// catalog declaration order.
func (c MaterialsCatalog) OptionsFor(attributeCode string) []AttributeOption {
	attributeCode = canonicalAttribute(attributeCode)
	var options []AttributeOption
	for _, option := range c.Options {
		if canonicalAttribute(option.AttributeCode) == attributeCode {
			options = append(options, option)
		}
	}
	return options
}

// NaturalUnitsFor returns the UnitDefinitions allowed for family, with
// FamilyUnitPolicy.Suggested units first (preserving relative declaration
// order within each group) so a UI can default to the first result.
func (c MaterialsCatalog) NaturalUnitsFor(family string) []UnitDefinition {
	family = canonical(family)
	var suggested, other []UnitDefinition
	for _, policy := range c.UnitPolicies {
		if canonical(policy.FamilyCode) != family || !policy.Allowed {
			continue
		}
		for _, unit := range c.Units {
			if canonical(unit.Code) != canonical(policy.UnitCode) {
				continue
			}
			if policy.Suggested {
				suggested = append(suggested, unit)
			} else {
				other = append(other, unit)
			}
			break
		}
	}
	return append(suggested, other...)
}

// ValidOptions returns attributeCode's options narrowed by
// AttributeOptionRelation given the already-chosen values in current. If no
// relation connects attributeCode to anything already set in current, every
// catalog option for attributeCode is returned unconstrained.
func (c MaterialsCatalog) ValidOptions(attributeCode string, current []MaterialAttributeValue) []AttributeOption {
	code := canonicalAttribute(attributeCode)
	chosen := map[string]MaterialAttributeValue{}
	for _, value := range current {
		chosen[canonicalAttribute(value.AttributeCode)] = value
	}
	allowed := map[string]bool{}
	constrained := false
	for _, relation := range c.Relations {
		from, to := canonicalAttribute(relation.FromAttribute), canonicalAttribute(relation.ToAttribute)
		if to == code {
			if value, ok := chosen[from]; ok && value.OptionCode == relation.FromOption {
				allowed[relation.ToOption] = true
				constrained = true
			}
		}
		if from == code {
			if value, ok := chosen[to]; ok && value.OptionCode == relation.ToOption {
				allowed[relation.FromOption] = true
				constrained = true
			}
		}
	}
	options := c.OptionsFor(code)
	if !constrained {
		return options
	}
	var narrowed []AttributeOption
	for _, option := range options {
		if allowed[option.Code] {
			narrowed = append(narrowed, option)
		}
	}
	return narrowed
}

// Effective resolves a's mode, identity participation, and applicability
// given current — the same conditional-rule evaluation NewMaterial already
// applies internally, exposed so a UI can decide whether to ask about this
// attribute at all before the user submits anything.
func (a FamilyAttribute) Effective(current []MaterialAttributeValue) (mode AttributeMode, identityParticipates, notApplicable bool) {
	byCode := map[string]MaterialAttributeValue{}
	for _, value := range current {
		byCode[canonicalAttribute(value.AttributeCode)] = value
	}
	return a.effective(byCode)
}
