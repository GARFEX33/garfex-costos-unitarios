package domain

import (
	"fmt"
	"sort"
	"strings"
)

func NewMaterial(catalog MaterialsCatalog, family, productType, naturalUnit string, values []MaterialAttributeValue) (Material, error) {
	family = canonical(family)
	if !catalog.hasFamily(family) {
		return Material{}, validation("family %q is not defined", family)
	}
	productType = canonical(productType)
	if !catalog.hasProductType(family, productType) {
		return Material{}, validation("product type %q is not defined for family %q", productType, family)
	}
	naturalUnit = canonical(naturalUnit)
	if !catalog.allowedUnit(family, naturalUnit) {
		return Material{}, validation("natural unit %q is not allowed for family %q", naturalUnit, family)
	}
	attributes := catalog.familyAttributes(family, productType)
	byCode := map[string]MaterialAttributeValue{}
	for _, value := range values {
		code := canonicalAttribute(value.AttributeCode)
		if _, exists := byCode[code]; exists {
			return Material{}, validation("attribute %q is repeated", code)
		}
		attribute, ok := attributes[code]
		if !ok {
			return Material{}, validation("attribute %q is not defined for family %q", code, family)
		}
		canonicalValue, err := catalog.canonicalValue(attribute.Definition, value)
		if err != nil {
			return Material{}, err
		}
		byCode[code] = canonicalValue
	}
	if err := catalog.validateRelations(byCode); err != nil {
		return Material{}, err
	}
	identity := make([]string, 0, len(attributes))
	canonicalValues := make([]MaterialAttributeValue, 0, len(byCode))
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
			return Material{}, validation("required attribute %q is missing", code)
		}
		if mode == ModeForbidden && present {
			return Material{}, validation("attribute %q is forbidden", code)
		}
		if mode == ModeConditional {
			return Material{}, validation("conditional attribute %q has no applicable rule", code)
		}
		if present {
			if notApplicable {
				return Material{}, validation("attribute %q must be not applicable", code)
			}
			canonicalValues = append(canonicalValues, value)
			if participates {
				identity = append(identity, code+"="+value.canonical(attribute.Definition))
			}
		}
	}
	sort.Slice(canonicalValues, func(i, j int) bool { return canonicalValues[i].AttributeCode < canonicalValues[j].AttributeCode })
	sort.Strings(identity)
	return Material{FamilyCode: family, ProductTypeCode: productType, NaturalUnit: naturalUnit, Attributes: canonicalValues, IdentityKey: family + "|" + productType + "|" + strings.Join(identity, "|")}, nil
}

func (c MaterialsCatalog) hasFamily(code string) bool {
	for _, family := range c.Families {
		if canonical(family.Code) == code {
			return true
		}
	}
	return false
}

func (c MaterialsCatalog) hasProductType(family, code string) bool {
	for _, productType := range c.ProductTypes {
		if canonical(productType.FamilyCode) == family && canonical(productType.Code) == code {
			return true
		}
	}
	return false
}

func (c MaterialsCatalog) allowedUnit(family, unit string) bool {
	for _, policy := range c.UnitPolicies {
		if canonical(policy.FamilyCode) == family && canonical(policy.UnitCode) == unit && policy.Allowed {
			return c.hasUnit(unit)
		}
	}
	return false
}

func (c MaterialsCatalog) hasUnit(code string) bool {
	for _, unit := range c.Units {
		if canonical(unit.Code) == code {
			return true
		}
	}
	return false
}

func (c MaterialsCatalog) familyAttributes(family, productType string) map[string]FamilyAttribute {
	result := map[string]FamilyAttribute{}
	for _, attribute := range c.FamilyAttributes {
		if canonical(attribute.FamilyCode) != family {
			continue
		}
		scope := canonical(attribute.ProductTypeCode)
		if scope != "" && scope != productType {
			continue
		}
		result[canonicalAttribute(attribute.Definition.Code)] = attribute
	}
	return result
}

func (c MaterialsCatalog) validateRelations(values map[string]MaterialAttributeValue) error {
	groups := map[string][]AttributeOptionRelation{}
	for _, relation := range c.Relations {
		key := canonicalAttribute(relation.FromAttribute) + "|" + canonicalAttribute(relation.ToAttribute)
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

func (a FamilyAttribute) effective(values map[string]MaterialAttributeValue) (AttributeMode, bool, bool) {
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
	return fmt.Errorf("%w: %s", ErrMaterialValidation, fmt.Sprintf(format, args...))
}
