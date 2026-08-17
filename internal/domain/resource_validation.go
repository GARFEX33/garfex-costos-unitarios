package domain

import (
	"fmt"
	"sort"
	"strings"
)

// NewResource validates values against catalog for scope's class/family/type
// and naturalUnit, and returns the canonical Resource — including its
// deterministic IdentityKey, now class-prefixed (design: "class|family|type|
// sorted(code=canonicalValue...)").
func NewResource(catalog ResourceCatalog, scope ResourceScope, naturalUnit string, values []ResourceAttributeValue) (Resource, error) {
	scope = scope.canonicalize()
	if !catalog.hasFamily(scope) {
		return Resource{}, validation("family %q is not defined for class %q", scope.FamilyCode, scope.ClassCode)
	}
	if !catalog.hasType(scope) {
		return Resource{}, validation("type %q is not defined for family %q", scope.TypeCode, scope.FamilyCode)
	}
	naturalUnit = canonical(naturalUnit)
	if !catalog.allowedUnit(scope, naturalUnit) {
		return Resource{}, validation("natural unit %q is not allowed for family %q", naturalUnit, scope.FamilyCode)
	}
	attributes := catalog.resourceAttributes(scope)
	byCode := map[string]ResourceAttributeValue{}
	for _, value := range values {
		code := canonicalAttribute(value.AttributeCode)
		if _, exists := byCode[code]; exists {
			return Resource{}, validation("attribute %q is repeated", code)
		}
		attribute, ok := attributes[code]
		if !ok {
			return Resource{}, validation("attribute %q is not defined for family %q", code, scope.FamilyCode)
		}
		canonicalValue, err := catalog.canonicalValue(attribute, value)
		if err != nil {
			return Resource{}, err
		}
		byCode[code] = canonicalValue
	}
	if err := catalog.validateRelations(attributes, byCode); err != nil {
		return Resource{}, err
	}
	identity := make([]string, 0, len(attributes)*3)
	canonicalValues := make([]ResourceAttributeValue, 0, len(byCode))
	codes := make([]string, 0, len(attributes))
	for code := range attributes {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		attribute := attributes[code]
		value, present := byCode[code]
		mode, participates, notApplicable := attribute.effective(byCode)
		if mode == ModeRequired && !present {
			return Resource{}, validation("required attribute %q is missing", code)
		}
		if mode == ModeForbidden && present {
			return Resource{}, validation("attribute %q is forbidden", code)
		}
		if mode == ModeConditional {
			return Resource{}, validation("conditional attribute %q has no applicable rule", code)
		}
		if present {
			if notApplicable {
				return Resource{}, validation("attribute %q must be not applicable", code)
			}
			canonicalValues = append(canonicalValues, value)
			if participates {
				identity = append(identity, identityComponent(code), identityComponent(string(value.Type)), identityComponent(value.canonical(attribute.Definition)))
			}
		}
	}
	sort.Slice(canonicalValues, func(i, j int) bool { return canonicalValues[i].AttributeCode < canonicalValues[j].AttributeCode })
	return Resource{
		ClassCode:   scope.ClassCode,
		FamilyCode:  scope.FamilyCode,
		TypeCode:    scope.TypeCode,
		NaturalUnit: naturalUnit,
		Attributes:  canonicalValues,
		IdentityKey: "v1|" + identityComponent(scope.ClassCode) + identityComponent(scope.FamilyCode) + identityComponent(scope.TypeCode) + strings.Join(identity, ""),
		canonical:   true,
	}, nil
}

func identityComponent(value string) string {
	return fmt.Sprintf("%d:%s", len([]byte(value)), value)
}

func RehydrateResource(catalog ResourceCatalog, snapshot ResourceSnapshot) (Resource, error) {
	if snapshot.ID <= 0 {
		return Resource{}, validation("resource snapshot ID must be positive")
	}
	var applicable []ResourceAttributeValue
	for _, value := range snapshot.Attributes {
		if value.Text != NotApplicableText {
			applicable = append(applicable, value)
		}
	}
	resource, err := NewResource(catalog, ResourceScope{ClassCode: snapshot.ClassCode, FamilyCode: snapshot.FamilyCode, TypeCode: snapshot.TypeCode}, snapshot.NaturalUnit, applicable)
	if err != nil {
		return Resource{}, err
	}
	if snapshot.IdentityKey != resource.IdentityKey {
		return Resource{}, validation("stored identity %q does not match canonical identity %q", snapshot.IdentityKey, resource.IdentityKey)
	}
	resource.ID = snapshot.ID
	for _, value := range snapshot.Attributes {
		if value.Text == NotApplicableText {
			resource.Attributes = append(resource.Attributes, value)
		}
	}
	sort.Slice(resource.Attributes, func(i, j int) bool {
		return resource.Attributes[i].AttributeCode < resource.Attributes[j].AttributeCode
	})
	return resource, nil
}

// hasFamily reports whether scope's FamilyCode is defined within scope's
// ClassCode — a family owned by a different class never matches (spec:
// "Family valid only within its class").
func (c ResourceCatalog) hasFamily(s ResourceScope) bool {
	s = s.canonicalize()
	for _, family := range c.Families {
		if canonical(family.ClassCode) == s.ClassCode && canonical(family.Code) == s.FamilyCode {
			return true
		}
	}
	return false
}

// hasType reports whether scope's TypeCode is defined within scope's
// class+family — a type owned by a different family never matches (spec:
// "Type valid only within its family+class").
func (c ResourceCatalog) hasType(s ResourceScope) bool {
	s = s.canonicalize()
	if s.TypeCode == "" {
		return false
	}
	for _, t := range c.Types {
		if canonical(t.ClassCode) == s.ClassCode && canonical(t.FamilyCode) == s.FamilyCode && canonical(t.Code) == s.TypeCode {
			return true
		}
	}
	return false
}

func (c ResourceCatalog) allowedUnit(s ResourceScope, unit string) bool {
	s = s.canonicalize()
	unit = canonical(unit)
	for _, policy := range c.UnitPolicies {
		if canonical(policy.ClassCode) == s.ClassCode && canonical(policy.FamilyCode) == s.FamilyCode && canonical(policy.UnitCode) == unit && policy.Allowed {
			return c.hasUnit(unit)
		}
	}
	return false
}

func (c ResourceCatalog) hasUnit(code string) bool {
	code = canonical(code)
	for _, unit := range c.Units {
		if canonical(unit.Code) == code {
			return true
		}
	}
	return false
}

// resourceAttributes returns the ResourceAttributes applicable to scope
// (both family-scoped and type-specific), keyed by canonical attribute
// code, using ResourceScope.matches's wildcard semantics: a catalog row
// with TypeCode == "" is shared by every Tipo of its Familia.
func (c ResourceCatalog) resourceAttributes(s ResourceScope) map[string]ResourceAttribute {
	result := map[string]ResourceAttribute{}
	for _, attribute := range c.Attributes {
		if !s.matches(attribute.ClassCode, attribute.FamilyCode, attribute.TypeCode) {
			continue
		}
		result[canonicalAttribute(attribute.Definition.Code)] = attribute
	}
	return result
}

// validateRelations checks value coherence for every AttributeOptionRelation
// whose two endpoints (a) are both present in attributes (this scope's
// resolved attribute set) and (b) both resolve to the SAME OptionSet as the
// relation itself (design D3/R4) — a relation declared in one class's option
// set never narrows an attribute bound to a different class's set.
func (c ResourceCatalog) validateRelations(attributes map[string]ResourceAttribute, values map[string]ResourceAttributeValue) error {
	groups := map[string][]AttributeOptionRelation{}
	for _, relation := range c.Relations {
		from, to := canonicalAttribute(relation.FromAttribute), canonicalAttribute(relation.ToAttribute)
		fromAttr, fromOK := attributes[from]
		toAttr, toOK := attributes[to]
		if !fromOK || !toOK {
			continue
		}
		set := canonicalOptionSet(relation.OptionSet)
		if set != fromAttr.setKey() || set != toAttr.setKey() {
			continue
		}
		key := from + "|" + to
		groups[key] = append(groups[key], relation)
	}
	for key, relations := range groups {
		parts := strings.Split(key, "|")
		from, fromOK := values[parts[0]]
		to, toOK := values[parts[1]]
		if !fromOK || !toOK {
			continue
		}
		valid := false
		for _, relation := range relations {
			if from.OptionCode == relation.FromOption && to.OptionCode == relation.ToOption {
				valid = true
				break
			}
		}
		if !valid {
			return validation("incoherent relation between %q and %q", parts[0], parts[1])
		}
	}
	return nil
}

func (a ResourceAttribute) effective(values map[string]ResourceAttributeValue) (AttributeMode, bool, bool) {
	for _, rule := range a.Rules {
		if value, ok := values[canonicalAttribute(rule.When.AttributeCode)]; ok && value.OptionCode == canonical(rule.When.Equals) {
			return rule.Mode, rule.IdentityParticipates, rule.NotApplicable
		}
	}
	if a.Mode == ModeConditional {
		return ModeRequired, a.IdentityParticipates, false
	}
	return a.Mode, a.IdentityParticipates, false
}

func validation(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrResourceValidation, fmt.Sprintf(format, args...))
}
