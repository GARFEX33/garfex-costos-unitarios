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
	Op     MutationOp
	Record CatalogRecord
}

var (
	// ErrCatalogKindUnknown is returned when a mutation names a
	// CatalogKindCode the registry does not recognize.
	ErrCatalogKindUnknown = errors.New("unknown catalog kind")
	// ErrCatalogRecordNotFound is returned when Update/Deactivate/
	// Reactivate/Delete cannot find a matching element by IdentityFields.
	ErrCatalogRecordNotFound = errors.New("catalog record not found")
	// ErrSoftDeleteUnsupported is returned when Deactivate/Reactivate
	// targets a kind whose underlying ResourceCatalog struct has no Active
	// field yet (see CatalogKind.SoftDelete's doc comment).
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
	if _, ok := registry.Kind(m.Record.Kind); !ok {
		return c, fmt.Errorf("%w: %q", ErrCatalogKindUnknown, m.Record.Kind)
	}

	next := c
	var err error
	switch m.Record.Kind {
	case KindClass:
		next.Classes, err = mutateSlice(c.Classes, m.Op,
			func(v ResourceClass) bool { return canonical(v.Code) == canonical(text(m.Record, "code")) },
			func() ResourceClass { return classFromRecord(m.Record) },
			func(v *ResourceClass, active bool) { v.Active = active },
		)
	case KindFamily:
		next.Families, err = mutateSlice(c.Families, m.Op,
			func(v ResourceFamily) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) && canonical(v.Code) == canonical(text(m.Record, "code"))
			},
			func() ResourceFamily { return familyFromRecord(m.Record) },
			func(v *ResourceFamily, active bool) { v.Active = active },
		)
	case KindType:
		next.Types, err = mutateSlice(c.Types, m.Op,
			func(v ResourceType) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) &&
					canonical(v.FamilyCode) == canonical(ref(m.Record, "family")) &&
					canonical(v.Code) == canonical(text(m.Record, "code"))
			},
			func() ResourceType { return typeFromRecord(m.Record) },
			func(v *ResourceType, active bool) { v.Active = active },
		)
	case KindAttributeDefinition:
		next.Definitions, err = mutateSlice(c.Definitions, m.Op,
			func(v AttributeDefinition) bool {
				return canonicalAttribute(v.Code) == canonicalAttribute(text(m.Record, "code"))
			},
			func() AttributeDefinition { return definitionFromRecord(m.Record) },
			nil, // no Active field on this Go struct yet (see CatalogKind.SoftDelete doc)
		)
	case KindOptionSet:
		// No ResourceCatalog slice represents named option sets: OptionSet
		// is a denormalized string tag on Options/Attributes/Relations, not
		// its own entity in the pure domain snapshot (design §3's own
		// File Changes table adds no such domain file). Nothing to mutate
		// here — the repository (a later PR) persists resource_option_sets
		// rows independently of this snapshot.
		return c, nil
	case KindOption:
		next.Options, err = mutateSlice(c.Options, m.Op,
			func(v AttributeOption) bool {
				return canonicalOptionSet(v.OptionSet) == canonicalOptionSet(ref(m.Record, "optionSet")) &&
					canonicalAttribute(v.AttributeCode) == canonicalAttribute(ref(m.Record, "characteristic")) &&
					v.Code == text(m.Record, "code")
			},
			func() AttributeOption { return optionFromRecord(m.Record) },
			func(v *AttributeOption, active bool) { v.Active = active },
		)
	case KindOptionRelation:
		next.Relations, err = mutateSlice(c.Relations, m.Op,
			func(v AttributeOptionRelation) bool {
				return canonicalOptionSet(v.OptionSet) == canonicalOptionSet(ref(m.Record, "optionSet")) &&
					v.FromOption == ref(m.Record, "fromOption") && v.ToOption == ref(m.Record, "toOption")
			},
			func() AttributeOptionRelation { return optionRelationFromRecord(m.Record) },
			nil,
		)
	case KindUnit:
		next.Units, err = mutateSlice(c.Units, m.Op,
			func(v UnitDefinition) bool { return canonical(v.Code) == canonical(text(m.Record, "code")) },
			func() UnitDefinition { return unitFromRecord(m.Record) },
			func(v *UnitDefinition, active bool) { v.Active = active },
		)
	case KindUnitPolicy:
		next.UnitPolicies, err = mutateSlice(c.UnitPolicies, m.Op,
			func(v ResourceUnitPolicy) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) &&
					canonical(v.FamilyCode) == canonical(ref(m.Record, "family")) &&
					canonical(v.UnitCode) == canonical(ref(m.Record, "unit"))
			},
			func() ResourceUnitPolicy { return unitPolicyFromRecord(m.Record) },
			nil,
		)
	case KindAttributeBinding:
		next.Attributes, err = mutateSlice(c.Attributes, m.Op,
			func(v ResourceAttribute) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) &&
					canonical(v.FamilyCode) == canonical(ref(m.Record, "family")) &&
					canonical(v.TypeCode) == canonical(ref(m.Record, "type")) &&
					canonicalAttribute(v.Definition.Code) == canonicalAttribute(ref(m.Record, "characteristic"))
			},
			func() ResourceAttribute { return c.attributeBindingFromRecord(m.Record) },
			nil,
		)
	case KindPresentationField:
		next.PresentationFields, err = mutateSlice(c.PresentationFields, m.Op,
			func(v PresentationField) bool {
				return canonical(v.ClassCode) == canonical(ref(m.Record, "class")) &&
					canonical(v.FamilyCode) == canonical(ref(m.Record, "family")) &&
					canonical(v.TypeCode) == canonical(ref(m.Record, "type")) &&
					canonicalAttribute(v.AttributeCode) == canonicalAttribute(ref(m.Record, "characteristic"))
			},
			func() PresentationField { return presentationFieldFromRecord(m.Record) },
			nil,
		)
	default:
		return c, fmt.Errorf("%w: %q", ErrCatalogKindUnknown, m.Record.Kind)
	}
	if err != nil {
		return c, err
	}
	return next, nil
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
	}
}

func unitFromRecord(rec CatalogRecord) UnitDefinition {
	return UnitDefinition{Code: text(rec, "code"), Symbol: text(rec, "symbol"), Dimension: text(rec, "dimension"), Active: rec.Active}
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
		ToAttribute: ref(rec, "toCharacteristic"), ToOption: ref(rec, "toOption"),
	}
}

func unitPolicyFromRecord(rec CatalogRecord) ResourceUnitPolicy {
	return ResourceUnitPolicy{
		ClassCode: ref(rec, "class"), FamilyCode: ref(rec, "family"), UnitCode: ref(rec, "unit"),
		Allowed: boolean(rec, "allowed"), Suggested: boolean(rec, "suggested"),
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
	}
}

func presentationFieldFromRecord(rec CatalogRecord) PresentationField {
	return PresentationField{
		ClassCode: ref(rec, "class"), FamilyCode: ref(rec, "family"), TypeCode: ref(rec, "type"),
		AttributeCode: ref(rec, "characteristic"), Position: integer(rec, "position"),
	}
}

// --- generic copy-on-write slice engine --------------------------------

// mutateSlice applies op to slice without ever modifying slice's own backing
// array: every branch builds a brand new slice before returning it. setActive
// may be nil for kinds with no Go Active field yet (Deactivate/Reactivate
// then return ErrSoftDeleteUnsupported).
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
		if setActive == nil {
			return nil, ErrSoftDeleteUnsupported
		}
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
