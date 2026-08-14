package domain

import "sort"

// defaultOptionSet is the sentinel canonicalOptionSet normalizes "" to
// (design D3/R3): an attribute or option with no explicit OptionSet is
// drawn from the shared default set.
const defaultOptionSet = "DEFAULT"

func canonicalOptionSet(s string) string {
	s = canonical(s)
	if s == "" {
		return defaultOptionSet
	}
	return s
}

// setKey resolves a's own OptionSet (canonicalized, defaulted) — the value
// OptionsFor/ValidOptions narrow by (design D3).
func (a ResourceAttribute) setKey() string { return canonicalOptionSet(a.OptionSet) }

// AttributesFor returns the ResourceAttributes applicable to scope (both
// family-scoped and type-specific), in the catalog's own declaration order
// — the order a UI should ask about them in. An unresolvable scope simply
// yields an empty result, not an error: this is a read query, not a
// validator (see NewResource for validation).
func (c ResourceCatalog) AttributesFor(s ResourceScope) []ResourceAttribute {
	s = s.canonicalize()
	var attributes []ResourceAttribute
	for _, attribute := range c.Attributes {
		if !s.matches(attribute.ClassCode, attribute.FamilyCode, attribute.TypeCode) {
			continue
		}
		attributes = append(attributes, attribute)
	}
	return attributes
}

// OptionsFor returns every catalog-controlled option in attr's own named
// OptionSet, in catalog declaration order. Two classes sharing an attribute
// code but naming DIFFERENT OptionSets never see each other's options;
// naming the SAME OptionSet explicitly shares one option list (design D3).
func (c ResourceCatalog) OptionsFor(attr ResourceAttribute) []AttributeOption {
	code := canonicalAttribute(attr.Definition.Code)
	set := attr.setKey()
	var options []AttributeOption
	for _, option := range c.Options {
		if !option.Active {
			continue
		}
		if canonicalAttribute(option.AttributeCode) == code && canonicalOptionSet(option.OptionSet) == set {
			options = append(options, option)
		}
	}
	return options
}

// NaturalUnitsFor returns the UnitDefinitions allowed for scope's family
// (TypeCode is ignored), with ResourceUnitPolicy.Suggested units first
// (preserving relative declaration order within each group) so a UI can
// default to the first result.
func (c ResourceCatalog) NaturalUnitsFor(s ResourceScope) []UnitDefinition {
	s = s.canonicalize()
	var suggested, other []UnitDefinition
	for _, policy := range c.UnitPolicies {
		if canonical(policy.ClassCode) != s.ClassCode || canonical(policy.FamilyCode) != s.FamilyCode || !policy.Allowed {
			continue
		}
		for _, unit := range c.Units {
			if !unit.Active {
				continue
			}
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

// FamiliesFor returns the ResourceFamilies belonging to classCode, in
// catalog declaration order — the editor's Familia step (design §1).
func (c ResourceCatalog) FamiliesFor(classCode string) []ResourceFamily {
	classCode = canonical(classCode)
	var families []ResourceFamily
	for _, family := range c.Families {
		if !family.Active {
			continue
		}
		if canonical(family.ClassCode) == classCode {
			families = append(families, family)
		}
	}
	return families
}

// ActiveClasses returns the Active ResourceClasses, sorted by Order then
// Name (design §1) — the menu/palette's Clase step.
func (c ResourceCatalog) ActiveClasses() []ResourceClass {
	var classes []ResourceClass
	for _, class := range c.Classes {
		if class.Active {
			classes = append(classes, class)
		}
	}
	sort.Slice(classes, func(i, j int) bool {
		if classes[i].Order != classes[j].Order {
			return classes[i].Order < classes[j].Order
		}
		return classes[i].Name < classes[j].Name
	})
	return classes
}

// ValidOptions returns attr's options narrowed by AttributeOptionRelation
// given the already-chosen values in current. If no relation in attr's own
// OptionSet connects attr to anything already set in current, every
// catalog option for attr is returned unconstrained.
func (c ResourceCatalog) ValidOptions(attr ResourceAttribute, current []ResourceAttributeValue) []AttributeOption {
	code := canonicalAttribute(attr.Definition.Code)
	set := attr.setKey()
	chosen := map[string]ResourceAttributeValue{}
	for _, value := range current {
		chosen[canonicalAttribute(value.AttributeCode)] = value
	}
	allowed := map[string]bool{}
	constrained := false
	for _, relation := range c.Relations {
		if canonicalOptionSet(relation.OptionSet) != set {
			continue
		}
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
	options := c.OptionsFor(attr)
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

// TypesFor returns the ResourceTypes belonging to scope's class+family
// (TypeCode is ignored), in catalog declaration order.
func (c ResourceCatalog) TypesFor(s ResourceScope) []ResourceType {
	s = s.canonicalize()
	var types []ResourceType
	for _, t := range c.Types {
		if !t.Active {
			continue
		}
		if canonical(t.ClassCode) == s.ClassCode && canonical(t.FamilyCode) == s.FamilyCode {
			types = append(types, t)
		}
	}
	return types
}

// Effective resolves a's mode, identity participation, and applicability
// given current — the same conditional-rule evaluation NewResource already
// applies internally, exposed so a UI can decide whether to ask about this
// attribute at all before the user submits anything.
func (a ResourceAttribute) Effective(current []ResourceAttributeValue) (mode AttributeMode, identityParticipates, notApplicable bool) {
	byCode := map[string]ResourceAttributeValue{}
	for _, value := range current {
		byCode[canonicalAttribute(value.AttributeCode)] = value
	}
	return a.effective(byCode)
}
