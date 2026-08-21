package domain

import (
	"errors"
	"fmt"
)

// MutationOp is the write operation CatalogMutation carries (design §4).
type MutationOp uint8

const (
	OpInsert MutationOp = iota
	OpUpdate
	OpDeactivate
	OpReactivate
	OpDelete
)

// CatalogMutation is one pending write against a ResourceCatalog snapshot
// (design §4) — the input ApplyCatalogMutation validates and applies.
type CatalogMutation struct {
	Op          MutationOp
	Record      CatalogRecord
	Replacement *CatalogRecord
}

var (
	// ErrCatalogKindUnknown is returned when a mutation names a
	// CatalogKindCode the registry does not recognize.
	ErrCatalogKindUnknown = errors.New("unknown catalog kind")
	// ErrCatalogRecordNotFound is returned when Update/Deactivate/
	// Reactivate/Delete cannot find a matching element by IdentityFields.
	ErrCatalogRecordNotFound = errors.New("catalog record not found")
	// ErrSoftDeleteUnsupported remains for source compatibility with legacy
	// callers; no registered kind returns it now that lifecycle support is
	// complete.
	ErrSoftDeleteUnsupported = errors.New("catalog kind does not support deactivate/reactivate")
	// ErrMutationOpUnknown is returned for an out-of-range MutationOp.
	ErrMutationOpUnknown = errors.New("unknown catalog mutation op")
)

// ApplyCatalogMutation applies m to c and returns a NEW ResourceCatalog —
// c itself is never mutated (design §4): every affected slice is copied
// before being modified, and the original c's slices/elements are left
// exactly as they were. It is a pure structural transform, not a validator
// — call the returned catalog's Validate() separately (design D9) before
// persisting.
func ApplyCatalogMutation(c ResourceCatalog, registry CatalogRegistry, m CatalogMutation) (ResourceCatalog, error) {
	m = cloneCatalogMutation(m)
	if _, ok := registry.Kind(m.Record.Kind); !ok {
		return c, fmt.Errorf("%w: %q", ErrCatalogKindUnknown, m.Record.Kind)
	}

	buildRecord := m.Record
	if m.Replacement != nil {
		buildRecord = *m.Replacement
	}
	next := cloneMutationCatalog(c)
	var err error
	switch m.Record.Kind {
	case KindClass:
		next.Classes, err = mutateSlice(c.Classes, m.Op,
			func(v ResourceClass) bool { return canonical(v.Code) == canonical(text(m.Record, "code")) },
			func() ResourceClass { return classFromRecord(buildRecord) },
			func(v *ResourceClass, active bool) { v.Active = active },
		)
	case KindFamily:
		next.Families, err = mutateSlice(c.Families, m.Op,
			func(v ResourceFamily) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) && canonical(v.Code) == canonical(text(m.Record, "code"))
			},
			func() ResourceFamily { return familyFromRecord(buildRecord) },
			func(v *ResourceFamily, active bool) { v.Active = active },
		)
	case KindType:
		next.Types, err = mutateSlice(c.Types, m.Op,
			func(v ResourceType) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) &&
					canonical(v.FamilyCode) == canonical(ref(m.Record, "family")) &&
					canonical(v.Code) == canonical(text(m.Record, "code"))
			},
			func() ResourceType { return typeFromRecord(buildRecord) },
			func(v *ResourceType, active bool) { v.Active = active },
		)
	case KindAttributeDefinition:
		next.Definitions, err = mutateSlice(c.Definitions, m.Op,
			func(v AttributeDefinition) bool {
				return canonicalAttribute(v.Code) == canonicalAttribute(text(m.Record, "code"))
			},
			func() AttributeDefinition { return definitionFromRecord(buildRecord) },
			func(v *AttributeDefinition, active bool) { v.Active = active },
		)
		if err == nil {
			code := canonicalAttribute(text(m.Record, "code"))
			switch m.Op {
			case OpUpdate:
				err = rebuildEmbeddedDefinitions(&next, code, definitionFromRecord(buildRecord))
			case OpDeactivate, OpReactivate:
				synchronizeEmbeddedDefinitionActive(&next, code)
			}
		}
	case KindOptionSet:
		next.OptionSets, err = mutateSlice(c.OptionSets, m.Op,
			func(v ResourceOptionSet) bool {
				return canonicalOptionSet(v.Code) == canonicalOptionSet(text(m.Record, "code"))
			},
			func() ResourceOptionSet { return optionSetFromRecord(buildRecord) },
			func(v *ResourceOptionSet, active bool) { v.Active = active },
		)
	case KindOption:
		next.Options, err = mutateSlice(c.Options, m.Op,
			func(v AttributeOption) bool {
				return canonicalOptionSet(v.OptionSet) == canonicalOptionSet(ref(m.Record, "optionSet")) &&
					canonicalAttribute(v.AttributeCode) == canonicalAttribute(ref(m.Record, "characteristic")) &&
					canonical(v.Code) == canonical(text(m.Record, "code"))
			},
			func() AttributeOption { return optionFromRecord(buildRecord) },
			func(v *AttributeOption, active bool) { v.Active = active },
		)
	case KindOptionRelation:
		next.Relations, err = mutateSlice(c.Relations, m.Op,
			func(v AttributeOptionRelation) bool {
				return canonicalOptionSet(v.OptionSet) == canonicalOptionSet(ref(m.Record, "optionSet")) &&
					canonical(v.FromOption) == canonical(ref(m.Record, "fromOption")) && canonical(v.ToOption) == canonical(ref(m.Record, "toOption"))
			},
			func() AttributeOptionRelation { return optionRelationFromRecord(buildRecord) },
			func(v *AttributeOptionRelation, active bool) { v.Active = active },
		)
	case KindUnit:
		next.Units, err = mutateSlice(c.Units, m.Op,
			func(v UnitDefinition) bool { return canonical(v.Code) == canonical(text(m.Record, "code")) },
			func() UnitDefinition { return unitFromRecord(buildRecord) },
			func(v *UnitDefinition, active bool) { v.Active = active },
		)
	case KindUnitPolicy:
		next.UnitPolicies, err = mutateSlice(c.UnitPolicies, m.Op,
			func(v ResourceUnitPolicy) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) &&
					canonical(v.FamilyCode) == canonical(ref(m.Record, "family")) &&
					canonical(v.UnitCode) == canonical(ref(m.Record, "unit"))
			},
			func() ResourceUnitPolicy { return unitPolicyFromRecord(buildRecord) },
			func(v *ResourceUnitPolicy, active bool) { v.Active = active },
		)
	case KindAttributeBinding:
		if m.Op == OpInsert || m.Op == OpUpdate {
			if err = validateApplicabilityRecord(c, buildRecord); err != nil {
				return c, err
			}
		}
		next.Attributes, err = mutateSlice(c.Attributes, m.Op,
			func(v ResourceAttribute) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) &&
					canonical(v.FamilyCode) == canonical(ref(m.Record, "family")) &&
					canonical(v.TypeCode) == canonical(ref(m.Record, "type")) &&
					canonicalAttribute(v.Definition.Code) == canonicalAttribute(ref(m.Record, "characteristic"))
			},
			func() ResourceAttribute { return c.attributeBindingFromRecord(buildRecord) },
			func(v *ResourceAttribute, active bool) { v.Active = active },
		)
	case KindPresentationField:
		next.PresentationFields, err = mutateSlice(c.PresentationFields, m.Op,
			func(v PresentationField) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) &&
					canonical(v.FamilyCode) == canonical(ref(m.Record, "family")) &&
					canonical(v.TypeCode) == canonical(ref(m.Record, "type")) &&
					canonicalAttribute(v.AttributeCode) == canonicalAttribute(ref(m.Record, "characteristic"))
			},
			func() PresentationField { return presentationFieldFromRecord(buildRecord) },
			func(v *PresentationField, active bool) { v.Active = active },
		)
	default:
		return c, fmt.Errorf("%w: %q", ErrCatalogKindUnknown, m.Record.Kind)
	}
	if err != nil {
		return c, err
	}
	return cloneMutationCatalog(next), nil
}

func cloneCatalogMutation(mutation CatalogMutation) CatalogMutation {
	mutation.Record = cloneCatalogRecord(mutation.Record)
	if mutation.Replacement != nil {
		replacement := cloneCatalogRecord(*mutation.Replacement)
		mutation.Replacement = &replacement
	}
	return mutation
}

func cloneMutationSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	copyOfValues := make([]T, len(values))
	copy(copyOfValues, values)
	return copyOfValues
}

func cloneMutationCatalog(catalog ResourceCatalog) ResourceCatalog {
	next := catalog
	next.Classes = cloneMutationSlice(catalog.Classes)
	for i := range next.Classes {
		next.Classes[i].Aliases = cloneCatalogStrings(catalog.Classes[i].Aliases)
		next.Classes[i].Keywords = cloneCatalogStrings(catalog.Classes[i].Keywords)
	}
	next.Families = cloneMutationSlice(catalog.Families)
	next.Types = cloneMutationSlice(catalog.Types)
	next.PresentationFields = cloneMutationSlice(catalog.PresentationFields)
	next.Units = cloneMutationSlice(catalog.Units)
	next.UnitPolicies = cloneMutationSlice(catalog.UnitPolicies)
	next.Definitions = cloneMutationSlice(catalog.Definitions)
	next.Attributes = cloneMutationSlice(catalog.Attributes)
	for i := range next.Attributes {
		next.Attributes[i].Rules = cloneMutationSlice(catalog.Attributes[i].Rules)
	}
	next.Options = cloneMutationSlice(catalog.Options)
	next.Relations = cloneMutationSlice(catalog.Relations)
	next.OptionSets = cloneMutationSlice(catalog.OptionSets)
	return next
}

// --- CatalogRecord field accessors -----------------------------------

func text(rec CatalogRecord, name string) string   { return rec.Values[name].Text }
func boolean(rec CatalogRecord, name string) bool  { return rec.Values[name].Bool }
func integer(rec CatalogRecord, name string) int   { return rec.Values[name].Int }
func list(rec CatalogRecord, name string) []string { return rec.Values[name].List }
func ref(rec CatalogRecord, name string) string    { return rec.Values[name].Ref.Code }

// --- CatalogRecord -> domain struct builders --------------------------

func classFromRecord(rec CatalogRecord) ResourceClass {
	return ResourceClass{
		Code: text(rec, "code"), Name: text(rec, "name"), Plural: text(rec, "plural"),
		Slug: text(rec, "slug"), Order: integer(rec, "order"),
		Aliases: list(rec, "aliases"), Keywords: list(rec, "keywords"),
		Active: rec.Active,
	}
}

func familyFromRecord(rec CatalogRecord) ResourceFamily {
	return ResourceFamily{ClassCode: ref(rec, "class"), Code: text(rec, "code"), Name: text(rec, "name"), Active: rec.Active}
}

func typeFromRecord(rec CatalogRecord) ResourceType {
	return ResourceType{ClassCode: ref(rec, "class"), FamilyCode: ref(rec, "family"), Code: text(rec, "code"), Name: text(rec, "name"), Active: rec.Active}
}

func definitionFromRecord(rec CatalogRecord) AttributeDefinition {
	return AttributeDefinition{
		Code: text(rec, "code"), Name: text(rec, "name"),
		ValueType:                   AttributeValueType(text(rec, "valueType")),
		Dimension:                   text(rec, "dimension"),
		DefaultIdentityParticipates: boolean(rec, "defaultIdentityParticipates"),
		Active:                      rec.Active,
	}
}

func optionSetFromRecord(rec CatalogRecord) ResourceOptionSet {
	return ResourceOptionSet{Code: text(rec, "code"), Name: text(rec, "name"), Active: rec.Active}
}

func unitFromRecord(rec CatalogRecord) UnitDefinition {
	return UnitDefinition{Code: text(rec, "code"), Name: text(rec, "name"), Symbol: text(rec, "symbol"), Dimension: text(rec, "dimension"), Active: rec.Active}
}

func optionFromRecord(rec CatalogRecord) AttributeOption {
	return AttributeOption{
		OptionSet: ref(rec, "optionSet"), AttributeCode: ref(rec, "characteristic"),
		Code: text(rec, "code"), Label: text(rec, "label"), Active: rec.Active,
	}
}

func optionRelationFromRecord(rec CatalogRecord) AttributeOptionRelation {
	return AttributeOptionRelation{
		OptionSet:     ref(rec, "optionSet"),
		FromAttribute: ref(rec, "fromCharacteristic"), FromOption: ref(rec, "fromOption"),
		ToAttribute: ref(rec, "toCharacteristic"), ToOption: ref(rec, "toOption"), Active: rec.Active,
	}
}

func unitPolicyFromRecord(rec CatalogRecord) ResourceUnitPolicy {
	return ResourceUnitPolicy{
		ClassCode: ref(rec, "class"), FamilyCode: ref(rec, "family"), UnitCode: ref(rec, "unit"),
		Allowed: boolean(rec, "allowed"), Suggested: boolean(rec, "suggested"), Active: rec.Active,
	}
}

// attributeBindingFromRecord resolves the full AttributeDefinition (not just
// its code) from c.Definitions via the existing definitionFor helper
// (resource_presentation.go), so a mutated ResourceAttribute carries the
// same fully-materialized Definition SeedResourceCatalog()'s own definition()
// helper produces — AttributesFor/Effective/resource_editor.go all read
// Definition.Name/.ValueType, not just .Code.
func (c ResourceCatalog) attributeBindingFromRecord(rec CatalogRecord) ResourceAttribute {
	return ResourceAttribute{
		ClassCode: ref(rec, "class"), FamilyCode: ref(rec, "family"), TypeCode: ref(rec, "type"),
		OptionSet:            ref(rec, "optionSet"),
		Definition:           c.definitionFor(ref(rec, "characteristic")),
		Mode:                 AttributeMode(text(rec, "mode")),
		IdentityParticipates: boolean(rec, "identityParticipates"),
		Rules:                attributeRulesFromRecords(rec.Rules),
		Active:               rec.Active,
	}
}

func presentationFieldFromRecord(rec CatalogRecord) PresentationField {
	return PresentationField{
		ClassCode: ref(rec, "class"), FamilyCode: ref(rec, "family"), TypeCode: ref(rec, "type"),
		AttributeCode: ref(rec, "characteristic"), Position: integer(rec, "position"), Active: rec.Active,
	}
}

func attributeRulesFromRecords(records []CatalogRuleRecord) []AttributeRule {
	if records == nil {
		return nil
	}
	rules := make([]AttributeRule, len(records))
	for i, record := range records {
		rules[i] = AttributeRule(record)
	}
	return rules
}

func synchronizeEmbeddedDefinitionActive(catalog *ResourceCatalog, code string) {
	var active bool
	for _, definition := range catalog.Definitions {
		if canonicalAttribute(definition.Code) == code {
			active = definition.Active
			break
		}
	}
	for i := range catalog.Attributes {
		if canonicalAttribute(catalog.Attributes[i].Definition.Code) == code {
			catalog.Attributes[i].Definition.Active = active
		}
	}
}

func rebuildEmbeddedDefinitions(catalog *ResourceCatalog, oldCode string, replacement AttributeDefinition) error {
	newCode := canonicalAttribute(replacement.Code)
	if newCode != oldCode && definitionHasReferences(*catalog, oldCode) {
		return fmt.Errorf("%w: definition %q is referenced", ErrCodeImmutable, oldCode)
	}
	if newCode == oldCode {
		for i := range catalog.Attributes {
			if canonicalAttribute(catalog.Attributes[i].Definition.Code) == oldCode {
				catalog.Attributes[i].Definition = replacement
			}
		}
	}
	return nil
}

func definitionHasReferences(catalog ResourceCatalog, code string) bool {
	for _, attribute := range catalog.Attributes {
		if canonicalAttribute(attribute.Definition.Code) == code {
			return true
		}
	}
	for _, option := range catalog.Options {
		if canonicalAttribute(option.AttributeCode) == code {
			return true
		}
	}
	for _, relation := range catalog.Relations {
		if canonicalAttribute(relation.FromAttribute) == code || canonicalAttribute(relation.ToAttribute) == code {
			return true
		}
	}
	for _, field := range catalog.PresentationFields {
		if canonicalAttribute(field.AttributeCode) == code {
			return true
		}
	}
	return false
}

func validateApplicabilityRecord(c ResourceCatalog, record CatalogRecord) error {
	if record.Rules == nil {
		return fmt.Errorf("%w: applicability rules are omitted", ErrResourceValidation)
	}
	classCode, err := requiredCatalogRef(record, "class", KindClass)
	if err != nil {
		return err
	}
	familyCode, err := requiredCatalogRef(record, "family", KindFamily)
	if err != nil {
		return err
	}
	if !c.hasFamilyIn(classCode, familyCode) {
		return fmt.Errorf("%w: applicability references unknown family %q", ErrResourceReference, familyCode)
	}
	if typeCode, err := optionalCatalogRef(record, "type", KindType); err != nil {
		return err
	} else if typeCode != "" && !c.hasTypeIn(classCode, familyCode, typeCode) {
		return fmt.Errorf("%w: applicability references unknown type %q", ErrResourceReference, typeCode)
	}
	characteristic, err := requiredCatalogRef(record, "characteristic", KindAttributeDefinition)
	if err != nil {
		return err
	}
	if !hasDefinition(c, characteristic) {
		return fmt.Errorf("%w: applicability references unknown characteristic %q", ErrResourceReference, characteristic)
	}
	if optionSet, err := optionalCatalogRef(record, "optionSet", KindOptionSet); err != nil {
		return err
	} else if optionSet != "" && !hasCatalogOptionSet(c, optionSet) {
		return fmt.Errorf("%w: applicability references unknown option set %q", ErrResourceReference, optionSet)
	}
	mode := AttributeMode(text(record, "mode"))
	if !validAttributeMode(mode) {
		return fmt.Errorf("%w: applicability mode is invalid", ErrResourceValidation)
	}
	if mode == ModeConditional && len(record.Rules) == 0 {
		return fmt.Errorf("%w: conditional applicability has no rules", ErrResourceValidation)
	}
	if mode != ModeConditional && len(record.Rules) > 0 {
		return fmt.Errorf("%w: non-conditional applicability has rules", ErrResourceValidation)
	}
	for _, rule := range record.Rules {
		if canonicalAttribute(rule.When.AttributeCode) == "" || canonical(rule.When.Equals) == "" || !validAttributeMode(rule.Mode) {
			return fmt.Errorf("%w: applicability rule is incomplete", ErrResourceValidation)
		}
		if !hasDefinition(c, rule.When.AttributeCode) {
			return fmt.Errorf("%w: applicability rule references unknown characteristic %q", ErrResourceReference, rule.When.AttributeCode)
		}
	}
	return nil
}

func requiredCatalogRef(record CatalogRecord, field string, kind CatalogKindCode) (string, error) {
	code, err := optionalCatalogRef(record, field, kind)
	if err != nil {
		return "", err
	}
	if code == "" {
		return "", fmt.Errorf("%w: applicability field %q is required", ErrResourceValidation, field)
	}
	return code, nil
}

func optionalCatalogRef(record CatalogRecord, field string, kind CatalogKindCode) (string, error) {
	value, ok := record.Values[field]
	if !ok {
		return "", nil
	}
	if value.Ref.Kind != "" && value.Ref.Kind != kind {
		return "", fmt.Errorf("%w: applicability field %q has the wrong reference kind", ErrResourceReference, field)
	}
	return value.Ref.Code, nil
}

func hasDefinition(c ResourceCatalog, code string) bool {
	code = canonicalAttribute(code)
	for _, definition := range c.Definitions {
		if canonicalAttribute(definition.Code) == code {
			return true
		}
	}
	return false
}

func hasCatalogOptionSet(c ResourceCatalog, code string) bool {
	code = canonicalOptionSet(code)
	for _, optionSet := range c.OptionSets {
		if canonicalOptionSet(optionSet.Code) == code {
			return true
		}
	}
	for _, option := range c.Options {
		if canonicalOptionSet(option.OptionSet) == code {
			return true
		}
	}
	return false
}

func validAttributeMode(mode AttributeMode) bool {
	return mode == ModeRequired || mode == ModeOptional || mode == ModeConditional || mode == ModeForbidden
}

// --- generic copy-on-write slice engine --------------------------------

// mutateSlice applies op to slice without ever modifying slice's own backing
// array: every branch builds a brand new slice before returning it. Every
// registered catalog kind supplies an active-state setter.
func mutateSlice[T any](slice []T, op MutationOp, matches func(T) bool, build func() T, setActive func(*T, bool)) ([]T, error) {
	switch op {
	case OpInsert:
		next := make([]T, len(slice), len(slice)+1)
		copy(next, slice)
		return append(next, build()), nil
	case OpUpdate:
		idx := indexOf(slice, matches)
		if idx < 0 {
			return nil, ErrCatalogRecordNotFound
		}
		next := append([]T(nil), slice...)
		next[idx] = build()
		return next, nil
	case OpDeactivate, OpReactivate:
		idx := indexOf(slice, matches)
		if idx < 0 {
			return nil, ErrCatalogRecordNotFound
		}
		next := append([]T(nil), slice...)
		setActive(&next[idx], op == OpReactivate)
		return next, nil
	case OpDelete:
		idx := indexOf(slice, matches)
		if idx < 0 {
			return nil, ErrCatalogRecordNotFound
		}
		next := make([]T, 0, len(slice)-1)
		next = append(next, slice[:idx]...)
		next = append(next, slice[idx+1:]...)
		return next, nil
	default:
		return nil, ErrMutationOpUnknown
	}
}

func indexOf[T any](slice []T, matches func(T) bool) int {
	for i, v := range slice {
		if matches(v) {
			return i
		}
	}
	return -1
}
