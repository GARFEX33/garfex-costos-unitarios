package domain

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrResourceValidation reports a structural defect ResourceCatalog.Validate
	// found in the catalog data itself (e.g. a duplicate or empty field),
	// as opposed to a bad narrowing reference (see ErrResourceReference).
	ErrResourceValidation = errors.New("resource catalog validation failed")
	// ErrResourceReference reports a dangling narrowing reference (a
	// ClassCode/FamilyCode/TypeCode/OptionSet/unit code that names
	// something the catalog does not define).
	ErrResourceReference = errors.New("resource catalog reference invalid")
)

// ResourceFamily is class-owned catalog data (recursos-maestro design §1,
// D1): a Familia only makes sense inside one Clase, so family codes are
// unique within a class, not globally.
type ResourceFamily struct {
	ClassCode string
	Code      string
	Name      string
	// Active mirrors ResourceClass.Active: a deactivated Familia is hidden from
	// active catalog choices while existing resources under it remain readable.
	Active bool
}

// ResourceType is the Tipo level, scoped to exactly one class+family.
type ResourceType struct {
	ClassCode  string
	FamilyCode string
	Code       string
	Name       string
	// Active — see ResourceFamily.Active.
	Active bool
}

// ResourceAttribute is the class/family/(type)-scoped attribute binding
// (design §2). TypeCode == "" means the attribute is shared by every Tipo of
// its Familia (see ResourceScope.matches). OptionSet names the named option
// set (design D3) this attribute's controlled values are drawn from.
type ResourceAttribute struct {
	ClassCode, FamilyCode, TypeCode string
	OptionSet                       string
	Definition                      AttributeDefinition
	Mode                            AttributeMode
	IdentityParticipates            bool
	Rules                           []AttributeRule
	Active                          bool
}

// ResourceUnitPolicy is class/family-scoped management-unit policy; it does
// not participate in resource identity.
type ResourceUnitPolicy struct {
	ClassCode, FamilyCode, UnitCode string
	Allowed, Suggested              bool
	Active                          bool
}

// Validate reports every structural defect in the catalog at once via
// errors.Join (design §1): a dangling Clase/Familia/Tipo/OptionSet
// reference, a UnitPolicy naming an unknown unit, a family code duplicated
// within one class (the same code reused under a DIFFERENT class is valid —
// D1), an empty class Name/Plural/Slug, and a duplicate class Code or Slug.
// It returns nil when the catalog is structurally sound.
func (c ResourceCatalog) Validate() error {
	var errs []error
	errs = append(errs, c.validateClasses()...)
	errs = append(errs, c.validateFamilies()...)
	errs = append(errs, c.validateTypes()...)
	errs = append(errs, c.validateAttributes()...)
	errs = append(errs, c.validateAttributeRules()...)
	errs = append(errs, c.validateUnits()...)
	errs = append(errs, c.validateUnitPolicies()...)
	errs = append(errs, c.validateActiveHierarchy()...)
	return errors.Join(errs...)
}

func (c ResourceCatalog) hasClassCode(code string) bool {
	code = canonical(code)
	for _, class := range c.Classes {
		if canonical(class.Code) == code {
			return true
		}
	}
	return false
}

func (c ResourceCatalog) hasFamilyIn(classCode, familyCode string) bool {
	classCode, familyCode = canonical(classCode), canonical(familyCode)
	for _, family := range c.Families {
		if canonical(family.ClassCode) == classCode && canonical(family.Code) == familyCode {
			return true
		}
	}
	return false
}

func (c ResourceCatalog) hasTypeIn(classCode, familyCode, typeCode string) bool {
	classCode, familyCode, typeCode = canonical(classCode), canonical(familyCode), canonical(typeCode)
	for _, t := range c.Types {
		if canonical(t.ClassCode) == classCode && canonical(t.FamilyCode) == familyCode && canonical(t.Code) == typeCode {
			return true
		}
	}
	return false
}

func (c ResourceCatalog) hasOptionSet(optionSet string) bool {
	optionSet = canonical(optionSet)
	for _, option := range c.Options {
		if canonical(option.OptionSet) == optionSet {
			return true
		}
	}
	return false
}

func (c ResourceCatalog) activeOptionSet(optionSet string) bool {
	if !c.activeSemanticsEnabled() {
		return true
	}
	set := canonicalOptionSet(optionSet)
	for _, optionSet := range c.OptionSets {
		if canonicalOptionSet(optionSet.Code) == set {
			return optionSet.Active
		}
	}
	return false
}

func (c ResourceCatalog) activeSemanticsEnabled() bool { return len(c.OptionSets) > 0 }

func (c ResourceCatalog) hasUnitCode(unitCode string) bool {
	unitCode = canonical(unitCode)
	for _, unit := range c.Units {
		if canonical(unit.Code) == unitCode {
			return true
		}
	}
	return false
}

func (c ResourceCatalog) validateClasses() []error {
	var errs []error
	seenCode := map[string]bool{}
	seenSlug := map[string]bool{}
	for _, class := range c.Classes {
		if strings.TrimSpace(class.Name) == "" {
			errs = append(errs, fmt.Errorf("%w: class %q has an empty name", ErrResourceValidation, class.Code))
		}
		if strings.TrimSpace(class.Plural) == "" {
			errs = append(errs, fmt.Errorf("%w: class %q has an empty plural", ErrResourceValidation, class.Code))
		}
		if strings.TrimSpace(class.Slug) == "" {
			errs = append(errs, fmt.Errorf("%w: class %q has an empty slug", ErrResourceValidation, class.Code))
		}
		if code := canonical(class.Code); code != "" {
			if seenCode[code] {
				errs = append(errs, fmt.Errorf("%w: duplicate class code %q", ErrResourceValidation, class.Code))
			}
			seenCode[code] = true
		}
		if slug := canonical(class.Slug); slug != "" {
			if seenSlug[slug] {
				errs = append(errs, fmt.Errorf("%w: duplicate class slug %q", ErrResourceValidation, class.Slug))
			}
			seenSlug[slug] = true
		}
	}
	return errs
}

func (c ResourceCatalog) validateFamilies() []error {
	var errs []error
	seen := map[string]bool{}
	for _, family := range c.Families {
		if !c.hasClassCode(family.ClassCode) {
			errs = append(errs, fmt.Errorf("%w: family %q references unknown class %q", ErrResourceReference, family.Code, family.ClassCode))
			continue
		}
		key := canonical(family.ClassCode) + "|" + canonical(family.Code)
		if seen[key] {
			errs = append(errs, fmt.Errorf("%w: duplicate family code %q within class %q", ErrResourceValidation, family.Code, family.ClassCode))
		}
		seen[key] = true
	}
	return errs
}

func (c ResourceCatalog) validateTypes() []error {
	var errs []error
	for _, t := range c.Types {
		if !c.hasFamilyIn(t.ClassCode, t.FamilyCode) {
			errs = append(errs, fmt.Errorf("%w: type %q references unknown family %q in class %q", ErrResourceReference, t.Code, t.FamilyCode, t.ClassCode))
		}
	}
	return errs
}

func (c ResourceCatalog) validateAttributes() []error {
	var errs []error
	for _, attribute := range c.Attributes {
		if !c.hasFamilyIn(attribute.ClassCode, attribute.FamilyCode) {
			errs = append(errs, fmt.Errorf("%w: attribute %q references unknown family %q in class %q", ErrResourceReference, attribute.Definition.Code, attribute.FamilyCode, attribute.ClassCode))
		} else if attribute.TypeCode != "" && !c.hasTypeIn(attribute.ClassCode, attribute.FamilyCode, attribute.TypeCode) {
			errs = append(errs, fmt.Errorf("%w: attribute %q references unknown type %q in family %q", ErrResourceReference, attribute.Definition.Code, attribute.TypeCode, attribute.FamilyCode))
		}
		if attribute.OptionSet != "" && !c.hasOptionSet(attribute.OptionSet) {
			errs = append(errs, fmt.Errorf("%w: attribute %q references unknown option set %q", ErrResourceReference, attribute.Definition.Code, attribute.OptionSet))
		}
	}
	return errs
}

// validateAttributeRules enforces that CONDITIONAL attributes have rules and
// other modes do not carry rules. Both directions are checked so every
// attribute has an effective mode.
func (c ResourceCatalog) validateAttributeRules() []error {
	var errs []error
	for _, attribute := range c.Attributes {
		hasRules := len(attribute.Rules) > 0
		switch {
		case attribute.Mode == ModeConditional && !hasRules:
			errs = append(errs, fmt.Errorf("%w: conditional attribute %q has no rules", ErrResourceValidation, attribute.Definition.Code))
		case attribute.Mode != ModeConditional && hasRules:
			errs = append(errs, fmt.Errorf("%w: non-conditional attribute %q must not have rules (mode %q is not conditional)", ErrResourceValidation, attribute.Definition.Code, attribute.Mode))
		}
	}
	return errs
}

// validateActiveHierarchy enforces design Risk#3's soft-delete structural
// invariant: an ACTIVE Familia can never sit under an INACTIVE Clase, and an
// ACTIVE Tipo can never sit under an INACTIVE Familia — deactivating a
// parent without also deactivating its active children is a structural
// inconsistency, not a valid intermediate state. An inactive child under an
// active parent is the normal, expected shape of Desactivar and is never
// flagged.
func (c ResourceCatalog) validateActiveHierarchy() []error {
	var errs []error
	classActive := map[string]bool{}
	for _, class := range c.Classes {
		classActive[canonical(class.Code)] = class.Active
	}
	familyActive := map[string]bool{}
	for _, family := range c.Families {
		key := canonical(family.ClassCode) + "|" + canonical(family.Code)
		familyActive[key] = family.Active
		if family.Active && !classActive[canonical(family.ClassCode)] {
			errs = append(errs, fmt.Errorf("%w: active family %q sits under inactive class %q", ErrResourceValidation, family.Code, family.ClassCode))
		}
	}
	for _, t := range c.Types {
		key := canonical(t.ClassCode) + "|" + canonical(t.FamilyCode)
		if t.Active && !familyActive[key] {
			errs = append(errs, fmt.Errorf("%w: active type %q sits under inactive family %q", ErrResourceValidation, t.Code, t.FamilyCode))
		}
	}
	return errs
}

func (c ResourceCatalog) validateUnitPolicies() []error {
	var errs []error
	for _, policy := range c.UnitPolicies {
		if !c.hasUnitCode(policy.UnitCode) {
			errs = append(errs, fmt.Errorf("%w: unit policy for family %q references unknown unit %q", ErrResourceReference, policy.FamilyCode, policy.UnitCode))
		}
	}
	return errs
}

func (c ResourceCatalog) validateUnits() []error {
	var errs []error
	seenCodes := map[string]bool{}
	seenSymbols := map[string]bool{}
	for _, unit := range c.Units {
		fields := []struct {
			name, value string
		}{
			{name: "code", value: unit.Code},
			{name: "name", value: unit.Name},
			{name: "symbol", value: unit.Symbol},
			{name: "dimension", value: unit.Dimension},
		}
		for _, field := range fields {
			if strings.TrimSpace(field.value) == "" {
				errs = append(errs, fmt.Errorf("%w: unit %s is blank", ErrResourceValidation, field.name))
			}
		}
		code := canonical(unit.Code)
		if code != "" && seenCodes[code] {
			errs = append(errs, fmt.Errorf("%w: duplicate unit code %q", ErrResourceValidation, unit.Code))
		}
		seenCodes[code] = true
		symbol := canonical(unit.Symbol)
		if symbol != "" && seenSymbols[symbol] {
			errs = append(errs, fmt.Errorf("%w: duplicate unit symbol %q", ErrResourceValidation, unit.Symbol))
		}
		seenSymbols[symbol] = true
	}
	return errs
}
