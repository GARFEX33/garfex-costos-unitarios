package domain

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// RED (task 3.3): ApplyCatalogMutation/CatalogMutation/MutationOp do not
// exist yet — every reference below fails to compile until
// catalog_mutation.go's GREEN lands.

func baseMutationCatalog() ResourceCatalog {
	return ResourceCatalog{
		Classes: []ResourceClass{
			{Code: "MATERIAL", Name: "Material", Plural: "Materiales", Slug: "materiales", Order: 1, Active: true},
		},
		Families: []ResourceFamily{
			{ClassCode: "MATERIAL", Code: "CONDUCTORES", Name: "Conductores", Active: true},
		},
	}
}

// TestApplyCatalogMutation_NeverMutatesInput is the load-bearing proof this
// task exists for: applying a mutation must never alter the ORIGINAL
// ResourceCatalog value passed in — only the returned value reflects it.
func TestCopyCatalogRecord_Table(t *testing.T) {
	tests := []struct {
		name  string
		value CatalogValue
		check func(t *testing.T, got CatalogValue)
	}{
		{
			name:  "text",
			value: CatalogValue{Text: "before"},
			check: func(t *testing.T, got CatalogValue) {
				if got.Text != "before" {
					t.Fatalf("copied text = %q, want before", got.Text)
				}
			},
		},
		{
			name:  "reference",
			value: CatalogValue{Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL", Label: "Material"}},
			check: func(t *testing.T, got CatalogValue) {
				if got.Ref != (CatalogRef{Kind: KindClass, Code: "MATERIAL", Label: "Material"}) {
					t.Fatalf("copied ref = %+v, want the original reference", got.Ref)
				}
			},
		},
		{
			name:  "list",
			value: CatalogValue{List: []string{"before"}},
			check: func(t *testing.T, got CatalogValue) {
				if len(got.List) != 1 || got.List[0] != "before" {
					t.Fatalf("copied list = %#v, want [before]", got.List)
				}
			},
		},
		{
			name:  "zero",
			value: CatalogValue{},
			check: func(t *testing.T, got CatalogValue) {
				if !reflect.DeepEqual(got, CatalogValue{}) {
					t.Fatalf("copied zero value = %+v, want zero value", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := CatalogRecord{Values: map[string]CatalogValue{"value": tt.value}}
			copy := cloneCatalogRecord(record)
			tt.check(t, copy.Values["value"])

			if len(record.Values["value"].List) > 0 {
				record.Values["value"].List[0] = "caller changed"
				if copy.Values["value"].List[0] == "caller changed" {
					t.Fatal("copy shares the caller's list backing array")
				}
			}
			record.Values["value"] = CatalogValue{Text: "caller changed"}
			if got := copy.Values["value"]; reflect.DeepEqual(got, CatalogValue{Text: "caller changed"}) {
				t.Fatal("copy shares the caller's values map")
			}
		})
	}
}

func TestMutationCopiesCallerAndPriorSnapshotStorage(t *testing.T) {
	catalog := ResourceCatalog{
		Classes:    []ResourceClass{{Code: "MATERIAL", Aliases: []string{"before"}, Keywords: []string{"keyword"}}},
		Attributes: []ResourceAttribute{{Rules: []AttributeRule{{When: AttributeCondition{AttributeCode: "color", Equals: "red"}}}}},
	}
	aliases := []string{"inserted"}
	mutation := CatalogMutation{
		Op: OpInsert,
		Record: CatalogRecord{
			Kind: KindClass,
			Values: map[string]CatalogValue{
				"code":     {Text: "TOOLS"},
				"aliases":  {List: aliases},
				"keywords": {List: []string{"tool"}},
			},
		},
	}

	next, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() error = %v, want nil", err)
	}

	aliases[0] = "caller changed"
	mutation.Record.Values["keywords"].List[0] = "caller changed"
	mutation.Record.Values["code"] = CatalogValue{Text: "caller changed"}
	if got := next.Classes[1]; got.Code != "TOOLS" || got.Aliases[0] != "inserted" || got.Keywords[0] != "tool" {
		t.Fatalf("candidate aliases caller storage = %+v, want isolated values", got)
	}

	next.Classes[0].Aliases[0] = "candidate changed"
	next.Classes[0].Keywords[0] = "candidate changed"
	next.Attributes[0].Rules[0].When.Equals = "candidate changed"
	if got := catalog.Classes[0]; got.Aliases[0] != "before" || got.Keywords[0] != "keyword" {
		t.Fatalf("prior class snapshot changed through candidate = %+v", got)
	}
	if got := catalog.Attributes[0].Rules[0].When.Equals; got != "red" {
		t.Fatalf("prior nested rule changed through candidate = %q, want red", got)
	}

	replacementAliases := []string{"replacement"}
	updated, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{
		Op: OpUpdate,
		Record: CatalogRecord{Kind: KindClass, Values: map[string]CatalogValue{
			"code": {Text: "MATERIAL"},
		}},
		Replacement: &CatalogRecord{Kind: KindClass, Values: map[string]CatalogValue{
			"code": {Text: "MATERIAL"}, "aliases": {List: replacementAliases},
		}},
	})
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() with replacement error = %v, want nil", err)
	}
	replacementAliases[0] = "replacement caller changed"
	if got := updated.Classes[0].Aliases[0]; got != "replacement" {
		t.Fatalf("replacement aliases = %#v, want isolated storage", updated.Classes[0].Aliases)
	}
}

func TestCatalogMutation_CopyRulesWithReplacementIsolation(t *testing.T) {
	rules := []CatalogRuleRecord{{
		When:                 AttributeCondition{AttributeCode: "insulation", Equals: "DESNUDO"},
		Mode:                 ModeForbidden,
		IdentityParticipates: true,
		NotApplicable:        true,
		Active:               true,
	}}
	replacementRules := []CatalogRuleRecord{{
		When:   AttributeCondition{AttributeCode: "insulation", Equals: "THW"},
		Mode:   ModeRequired,
		Active: false,
	}}
	mutation := CatalogMutation{
		Record:      CatalogRecord{Kind: KindAttributeBinding, Rules: rules},
		Replacement: &CatalogRecord{Kind: KindAttributeBinding, Rules: replacementRules},
	}
	copied := cloneCatalogMutation(mutation)

	rules[0].When.Equals = "caller changed"
	replacementRules[0].Mode = ModeOptional
	mutation.Record.Rules[0].NotApplicable = false
	mutation.Replacement.Rules[0].Active = true
	if copied.Record.Rules[0].When.Equals != "DESNUDO" || !copied.Record.Rules[0].NotApplicable {
		t.Fatalf("copied mutation record rules changed through caller storage: %#v", copied.Record.Rules)
	}
	if copied.Replacement.Rules[0].Mode != ModeRequired || copied.Replacement.Rules[0].Active {
		t.Fatalf("copied replacement rules changed through caller storage: %#v", copied.Replacement.Rules)
	}

	copied.Record.Rules[0].When.AttributeCode = "candidate changed"
	copied.Replacement.Rules[0].When.Equals = "candidate changed"
	if mutation.Record.Rules[0].When.AttributeCode != "insulation" || mutation.Replacement.Rules[0].When.Equals != "THW" {
		t.Fatalf("prior mutation changed through copied rules: record=%#v replacement=%#v", mutation.Record.Rules, mutation.Replacement.Rules)
	}
}

func TestApplyCatalogMutation_NeverMutatesInput(t *testing.T) {
	original := baseMutationCatalog()
	originalFamiliesLen := len(original.Families)
	originalFirstFamily := original.Families[0]

	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpInsert,
		Record: CatalogRecord{
			Kind: KindFamily,
			Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}},
				"code":  {Text: "CANALIZACIONES"},
				"name":  {Text: "Canalizaciones"},
			},
			Active: true,
		},
	}

	next, err := ApplyCatalogMutation(original, registry, mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() error = %v, want nil", err)
	}

	if len(original.Families) != originalFamiliesLen {
		t.Fatalf("original.Families length changed from %d to %d — input was mutated", originalFamiliesLen, len(original.Families))
	}
	if original.Families[0] != originalFirstFamily {
		t.Fatalf("original.Families[0] changed from %+v to %+v — input was mutated", originalFirstFamily, original.Families[0])
	}
	if len(next.Families) != originalFamiliesLen+1 {
		t.Fatalf("next.Families length = %d, want %d (the returned value must reflect the insert)", len(next.Families), originalFamiliesLen+1)
	}
}

// TestApplyCatalogMutation_InsertFamily triangulates Insert against a second
// kind's own field shape (Class ref + code + name), proving the generic
// engine is not hardcoded to one kind.
func TestApplyCatalogMutation_InsertFamily(t *testing.T) {
	catalog := baseMutationCatalog()
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpInsert,
		Record: CatalogRecord{
			Kind: KindFamily,
			Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}},
				"code":  {Text: "CANALIZACIONES"},
				"name":  {Text: "Canalizaciones"},
			},
			Active: true,
		},
	}

	next, err := ApplyCatalogMutation(catalog, registry, mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() error = %v, want nil", err)
	}
	if len(next.Families) != 2 {
		t.Fatalf("next.Families = %+v, want 2 entries", next.Families)
	}
	inserted := next.Families[1]
	if inserted.ClassCode != "MATERIAL" || inserted.Code != "CANALIZACIONES" || inserted.Name != "Canalizaciones" || !inserted.Active {
		t.Fatalf("inserted family = %+v, want {ClassCode:MATERIAL Code:CANALIZACIONES Name:Canalizaciones Active:true}", inserted)
	}
}

// TestApplyCatalogMutation_UpdateFamily proves Update replaces the matched
// element's mutable fields in the returned copy.
func TestApplyCatalogMutation_UpdateFamily(t *testing.T) {
	catalog := baseMutationCatalog()
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpUpdate,
		Record: CatalogRecord{
			Kind: KindFamily,
			Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}},
				"code":  {Text: "CONDUCTORES"},
				"name":  {Text: "Conductores eléctricos"},
			},
			Active: true,
		},
	}

	next, err := ApplyCatalogMutation(catalog, registry, mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() error = %v, want nil", err)
	}
	if len(next.Families) != 1 {
		t.Fatalf("next.Families = %+v, want exactly 1 entry (update, not insert)", next.Families)
	}
	if next.Families[0].Name != "Conductores eléctricos" {
		t.Fatalf("next.Families[0].Name = %q, want %q", next.Families[0].Name, "Conductores eléctricos")
	}
}

// TestApplyCatalogMutation_DeactivateRemovesFromUsableSnapshot proves
// Deactivate sets Active=false on the matched element and that FamiliesFor
// (task 3.5's own filter) then excludes it — the design's own explicit
// "deactivate removes from the usable snapshot" testing-strategy scenario.
func TestApplyCatalogMutation_DeactivateRemovesFromUsableSnapshot(t *testing.T) {
	catalog := baseMutationCatalog()
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpDeactivate,
		Record: CatalogRecord{
			Kind: KindFamily,
			Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}},
				"code":  {Text: "CONDUCTORES"},
			},
		},
	}

	next, err := ApplyCatalogMutation(catalog, registry, mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() error = %v, want nil", err)
	}
	if next.Families[0].Active {
		t.Fatalf("next.Families[0].Active = true, want false after Deactivate")
	}
	if got := next.FamiliesFor("MATERIAL"); len(got) != 0 {
		t.Fatalf("FamiliesFor(MATERIAL) after Deactivate = %+v, want empty (deactivated families are excluded from the usable snapshot)", got)
	}

	reactivated, err := ApplyCatalogMutation(next, registry, CatalogMutation{Op: OpReactivate, Record: mutation.Record})
	if err != nil {
		t.Fatalf("ApplyCatalogMutation(Reactivate) error = %v, want nil", err)
	}
	if got := reactivated.FamiliesFor("MATERIAL"); len(got) != 1 {
		t.Fatalf("FamiliesFor(MATERIAL) after Reactivate = %+v, want 1 entry", got)
	}
}

// TestApplyCatalogMutation_DeleteFamily proves Delete physically removes the
// matched element.
func TestApplyCatalogMutation_DeleteFamily(t *testing.T) {
	catalog := baseMutationCatalog()
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpDelete,
		Record: CatalogRecord{
			Kind: KindFamily,
			Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}},
				"code":  {Text: "CONDUCTORES"},
			},
		},
	}

	next, err := ApplyCatalogMutation(catalog, registry, mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() error = %v, want nil", err)
	}
	if len(next.Families) != 0 {
		t.Fatalf("next.Families = %+v, want empty after Delete", next.Families)
	}
	if len(catalog.Families) != 1 {
		t.Fatalf("original catalog.Families = %+v, want unchanged (still 1 entry) — Delete must not mutate the input", catalog.Families)
	}
}

// TestApplyCatalogMutation_UpdateUnknownRecordReturnsError triangulates the
// not-found path: Update/Deactivate/Reactivate/Delete against an identity
// that does not exist in the snapshot must fail, not silently no-op.
func TestApplyCatalogMutation_UpdateUnknownRecordReturnsError(t *testing.T) {
	catalog := baseMutationCatalog()
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpUpdate,
		Record: CatalogRecord{
			Kind: KindFamily,
			Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}},
				"code":  {Text: "NOPE"},
				"name":  {Text: "No existe"},
			},
		},
	}

	if _, err := ApplyCatalogMutation(catalog, registry, mutation); err == nil {
		t.Fatal("ApplyCatalogMutation(Update, unknown identity) error = nil, want an error")
	}
}

func TestApplyCatalogMutation_DeactivateDefinitionChangesActiveState(t *testing.T) {
	catalog := ResourceCatalog{Definitions: []AttributeDefinition{{Code: "color", Name: "Color", Active: true}}}
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpDeactivate,
		Record: CatalogRecord{
			Kind:   KindAttributeDefinition,
			Values: map[string]CatalogValue{"code": {Text: "color"}},
		},
	}

	next, err := ApplyCatalogMutation(catalog, registry, mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation(Deactivate, AttributeDefinition) error = %v, want nil", err)
	}
	if next.Definitions[0].Name != "Color" || next.Definitions[0].Active {
		t.Fatalf("deactivated definition = %+v, want fields preserved and Active=false", next.Definitions[0])
	}
}

// TestApplyCatalogMutation_InvalidMutationCaughtByValidate proves
// ApplyCatalogMutation's OUTPUT feeds correctly into Validate() (design §4):
// inserting a Familia that references an unknown Clase produces a catalog
// Validate() rejects, even though ApplyCatalogMutation itself succeeded (it
// is a pure structural transform, not a validator).
func TestApplyCatalogMutation_InvalidMutationCaughtByValidate(t *testing.T) {
	catalog := baseMutationCatalog()
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpInsert,
		Record: CatalogRecord{
			Kind: KindFamily,
			Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "GHOST"}},
				"code":  {Text: "X"},
				"name":  {Text: "X"},
			},
			Active: true,
		},
	}

	next, err := ApplyCatalogMutation(catalog, registry, mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() error = %v, want nil (structural validity is Validate()'s job, not ApplyCatalogMutation's)", err)
	}
	if err := next.Validate(); err == nil {
		t.Fatal("next.Validate() = nil, want an error (dangling class reference)")
	}
}

// TestApplyCatalogMutation_UnknownKindReturnsError triangulates the registry
// guard: a Kind not present in the registry is rejected up front.
func TestApplyCatalogMutation_UnknownKindReturnsError(t *testing.T) {
	catalog := baseMutationCatalog()
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{Op: OpInsert, Record: CatalogRecord{Kind: CatalogKindCode("NOPE")}}

	if _, err := ApplyCatalogMutation(catalog, registry, mutation); err == nil {
		t.Fatal("ApplyCatalogMutation(unknown kind) error = nil, want an error")
	}
}

func TestApplyCatalogMutation_FiveKindLosslessLifecycleMatrix(t *testing.T) {
	type materializerCase struct {
		name        string
		kind        CatalogKindCode
		insert      CatalogRecord
		replacement CatalogRecord
		assert      func(t *testing.T, record interface{}, expected CatalogRecord, active bool)
	}

	cases := []materializerCase{
		{
			name: string(KindClass), kind: KindClass,
			insert: CatalogRecord{Kind: KindClass, Active: true, Values: map[string]CatalogValue{
				"code": {Text: "MATERIAL"}, "name": {Text: "Material"}, "plural": {Text: "Materiales"},
				"slug": {Text: "materiales"}, "order": {Int: 7}, "aliases": {List: []string{"mat", "materia"}},
				"keywords": {List: []string{"steel", "cable"}},
			}},
			replacement: CatalogRecord{Kind: KindClass, Active: false, Values: map[string]CatalogValue{
				"code": {Text: "MATERIAL"}, "name": {Text: "Material actualizado"}, "plural": {Text: "Materiales actualizados"},
				"slug": {Text: "materiales-actualizados"}, "order": {Int: 8}, "aliases": {List: []string{"actualizado"}},
				"keywords": {List: []string{"nuevo"}},
			}},
			assert: func(t *testing.T, value interface{}, expected CatalogRecord, active bool) {
				got := value.(ResourceClass)
				if got.Code != text(expected, "code") || got.Name != text(expected, "name") || got.Plural != text(expected, "plural") || got.Slug != text(expected, "slug") || got.Order != integer(expected, "order") || !reflect.DeepEqual(got.Aliases, list(expected, "aliases")) || !reflect.DeepEqual(got.Keywords, list(expected, "keywords")) || got.Active != active {
					t.Fatalf("class = %+v, want every materialized field and Active=%t", got, active)
				}
			},
		},
		{
			name: string(KindFamily), kind: KindFamily,
			insert: CatalogRecord{Kind: KindFamily, Active: true, Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}}, "code": {Text: "CONDUCTORES"}, "name": {Text: "Conductores"},
			}},
			replacement: CatalogRecord{Kind: KindFamily, Active: false, Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}}, "code": {Text: "CONDUCTORES"}, "name": {Text: "Conductores actualizados"},
			}},
			assert: func(t *testing.T, value interface{}, expected CatalogRecord, active bool) {
				got := value.(ResourceFamily)
				if got.ClassCode != ref(expected, "class") || got.Code != text(expected, "code") || got.Name != text(expected, "name") || got.Active != active {
					t.Fatalf("family = %+v, want every materialized field and Active=%t", got, active)
				}
			},
		},
		{
			name: string(KindType), kind: KindType,
			insert: CatalogRecord{Kind: KindType, Active: true, Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}}, "family": {Ref: CatalogRef{Kind: KindFamily, Code: "CONDUCTORES"}},
				"code": {Text: "THHN"}, "name": {Text: "THHN"},
			}},
			replacement: CatalogRecord{Kind: KindType, Active: false, Values: map[string]CatalogValue{
				"class": {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}}, "family": {Ref: CatalogRef{Kind: KindFamily, Code: "CONDUCTORES"}},
				"code": {Text: "THHN"}, "name": {Text: "THHN actualizado"},
			}},
			assert: func(t *testing.T, value interface{}, expected CatalogRecord, active bool) {
				got := value.(ResourceType)
				if got.ClassCode != ref(expected, "class") || got.FamilyCode != ref(expected, "family") || got.Code != text(expected, "code") || got.Name != text(expected, "name") || got.Active != active {
					t.Fatalf("type = %+v, want every materialized field and Active=%t", got, active)
				}
			},
		},
		{
			name: string(KindAttributeDefinition), kind: KindAttributeDefinition,
			insert: CatalogRecord{Kind: KindAttributeDefinition, Active: true, Values: map[string]CatalogValue{
				"code": {Text: "color"}, "name": {Text: "Color"}, "valueType": {Text: "CONTROLLED_OPTION"},
				"dimension": {Text: "COLOR"}, "defaultIdentityParticipates": {Bool: true},
			}},
			replacement: CatalogRecord{Kind: KindAttributeDefinition, Active: false, Values: map[string]CatalogValue{
				"code": {Text: "color"}, "name": {Text: "Color actualizado"}, "valueType": {Text: "CONTROLLED_TEXT"},
				"dimension": {Text: "TEXT"}, "defaultIdentityParticipates": {Bool: false},
			}},
			assert: func(t *testing.T, value interface{}, expected CatalogRecord, active bool) {
				got := value.(AttributeDefinition)
				if got.Code != text(expected, "code") || got.Name != text(expected, "name") || got.ValueType != AttributeValueType(text(expected, "valueType")) || got.Dimension != text(expected, "dimension") || got.DefaultIdentityParticipates != boolean(expected, "defaultIdentityParticipates") || got.Active != active {
					t.Fatalf("definition = %+v, want every materialized field and Active=%t", got, active)
				}
			},
		},
		{
			name: string(KindOptionSet), kind: KindOptionSet,
			insert: CatalogRecord{Kind: KindOptionSet, Active: true, Values: map[string]CatalogValue{
				"code": {Text: "COLORS"}, "name": {Text: "Colores"},
			}},
			replacement: CatalogRecord{Kind: KindOptionSet, Active: false, Values: map[string]CatalogValue{
				"code": {Text: "COLORS"}, "name": {Text: "Colores actualizados"},
			}},
			assert: func(t *testing.T, value interface{}, expected CatalogRecord, active bool) {
				got := value.(ResourceOptionSet)
				if got.Code != text(expected, "code") || got.Name != text(expected, "name") || got.Active != active {
					t.Fatalf("option set = %+v, want every materialized field and Active=%t", got, active)
				}
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			catalog := ResourceCatalog{}
			registry := NewCatalogRegistry()
			next, err := ApplyCatalogMutation(catalog, registry, CatalogMutation{Op: OpInsert, Record: tt.insert})
			if err != nil {
				t.Fatalf("insert error = %v, want nil", err)
			}
			assertLastMaterialized(t, next, tt.kind, tt.insert, tt.insert.Active, tt.assert)

			updated, err := ApplyCatalogMutation(next, registry, CatalogMutation{Op: OpUpdate, Record: tt.insert, Replacement: &tt.replacement})
			if err != nil {
				t.Fatalf("update replacement error = %v, want nil", err)
			}
			assertLastMaterialized(t, updated, tt.kind, tt.replacement, tt.replacement.Active, tt.assert)

			inactive, err := ApplyCatalogMutation(updated, registry, CatalogMutation{Op: OpDeactivate, Record: tt.insert})
			if err != nil {
				t.Fatalf("deactivate error = %v, want nil", err)
			}
			assertLastMaterialized(t, inactive, tt.kind, tt.replacement, false, tt.assert)

			active, err := ApplyCatalogMutation(inactive, registry, CatalogMutation{Op: OpReactivate, Record: tt.insert})
			if err != nil {
				t.Fatalf("reactivate error = %v, want nil", err)
			}
			assertLastMaterialized(t, active, tt.kind, tt.replacement, true, tt.assert)

			deleted, err := ApplyCatalogMutation(active, registry, CatalogMutation{Op: OpDelete, Record: tt.insert})
			if err != nil {
				t.Fatalf("delete error = %v, want nil", err)
			}
			if collectionLength(deleted, tt.kind) != 0 {
				t.Fatalf("delete left %s records in candidate: %+v", tt.kind, deleted)
			}
			if collectionLength(catalog, tt.kind) != 0 {
				t.Fatalf("delete changed the prior snapshot for %s", tt.kind)
			}
		})
	}
}

func assertLastMaterialized(t *testing.T, catalog ResourceCatalog, kind CatalogKindCode, expected CatalogRecord, active bool, assert func(*testing.T, interface{}, CatalogRecord, bool)) {
	t.Helper()
	if collectionLength(catalog, kind) == 0 {
		t.Fatalf("%s candidate has no materialized records", kind)
	}
	switch kind {
	case KindClass:
		assert(t, catalog.Classes[len(catalog.Classes)-1], expected, active)
	case KindFamily:
		assert(t, catalog.Families[len(catalog.Families)-1], expected, active)
	case KindType:
		assert(t, catalog.Types[len(catalog.Types)-1], expected, active)
	case KindAttributeDefinition:
		assert(t, catalog.Definitions[len(catalog.Definitions)-1], expected, active)
	case KindOptionSet:
		assert(t, catalog.OptionSets[len(catalog.OptionSets)-1], expected, active)
	default:
		t.Fatalf("unsupported matrix kind %s", kind)
	}
}

func collectionLength(catalog ResourceCatalog, kind CatalogKindCode) int {
	switch kind {
	case KindClass:
		return len(catalog.Classes)
	case KindFamily:
		return len(catalog.Families)
	case KindType:
		return len(catalog.Types)
	case KindAttributeDefinition:
		return len(catalog.Definitions)
	case KindOptionSet:
		return len(catalog.OptionSets)
	default:
		return -1
	}
}

func TestApplyCatalogMutation_UnknownIdentityMatrix(t *testing.T) {
	cases := []struct {
		name   string
		kind   CatalogKindCode
		record CatalogRecord
	}{
		{name: string(KindClass), kind: KindClass, record: CatalogRecord{Kind: KindClass, Values: map[string]CatalogValue{"code": {Text: "MISSING"}}}},
		{name: string(KindFamily), kind: KindFamily, record: CatalogRecord{Kind: KindFamily, Values: map[string]CatalogValue{"class": {Ref: CatalogRef{Code: "MATERIAL"}}, "code": {Text: "MISSING"}}}},
		{name: string(KindType), kind: KindType, record: CatalogRecord{Kind: KindType, Values: map[string]CatalogValue{"class": {Ref: CatalogRef{Code: "MATERIAL"}}, "family": {Ref: CatalogRef{Code: "FAMILY"}}, "code": {Text: "MISSING"}}}},
		{name: string(KindAttributeDefinition), kind: KindAttributeDefinition, record: CatalogRecord{Kind: KindAttributeDefinition, Values: map[string]CatalogValue{"code": {Text: "MISSING"}}}},
		{name: string(KindOptionSet), kind: KindOptionSet, record: CatalogRecord{Kind: KindOptionSet, Values: map[string]CatalogValue{"code": {Text: "MISSING"}}}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			for _, op := range []MutationOp{OpUpdate, OpDeactivate, OpReactivate, OpDelete} {
				t.Run(fmt.Sprintf("op-%d", op), func(t *testing.T) {
					mutation := CatalogMutation{Op: op, Record: tt.record}
					if _, err := ApplyCatalogMutation(ResourceCatalog{}, NewCatalogRegistry(), mutation); !errors.Is(err, ErrCatalogRecordNotFound) {
						t.Fatalf("%s missing op %d error = %v, want ErrCatalogRecordNotFound", tt.kind, op, err)
					}
				})
			}
		})
	}
}

func TestApplyCatalogMutation_DeleteClassSliceBoundaries(t *testing.T) {
	original := ResourceCatalog{Classes: []ResourceClass{
		{Code: "FIRST", Name: "First", Plural: "First", Slug: "first", Active: true},
		{Code: "MIDDLE", Name: "Middle", Plural: "Middle", Slug: "middle", Active: false},
		{Code: "LAST", Name: "Last", Plural: "Last", Slug: "last", Active: true},
	}}
	for _, code := range []string{"FIRST", "MIDDLE", "LAST"} {
		t.Run(code, func(t *testing.T) {
			next, err := ApplyCatalogMutation(original, NewCatalogRegistry(), CatalogMutation{
				Op: OpDelete, Record: CatalogRecord{Kind: KindClass, Values: map[string]CatalogValue{"code": {Text: code}}},
			})
			if err != nil {
				t.Fatalf("delete %s error = %v, want nil", code, err)
			}
			if len(next.Classes) != 2 {
				t.Fatalf("delete %s left %d classes, want 2", code, len(next.Classes))
			}
			for _, class := range next.Classes {
				if class.Code == code {
					t.Fatalf("delete %s left deleted class in %+v", code, next.Classes)
				}
			}
			if len(original.Classes) != 3 {
				t.Fatalf("delete %s changed prior snapshot: %+v", code, original.Classes)
			}
		})
	}
}

func TestApplyCatalogMutation_UnitNameRoundTrip(t *testing.T) {
	catalog := validResourceCatalog()
	next, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpUpdate, Record: CatalogRecord{Kind: KindUnit, Values: map[string]CatalogValue{
		"code": {Text: "M"}, "name": {Text: "Metro lineal"}, "symbol": {Text: "M"}, "dimension": {Text: "LENGTH"},
	}}})
	if err != nil {
		t.Fatalf("ApplyCatalogMutation() error = %v, want nil", err)
	}
	if next.Units[0].Name != "Metro lineal" || next.Units[0].Code != "M" || next.Units[0].Symbol != "M" {
		t.Fatalf("mutated unit = %+v, want name round-trip with stable code/symbol", next.Units[0])
	}
}

func remainingKindCatalog() ResourceCatalog {
	return ResourceCatalog{
		Classes:     []ResourceClass{{Code: "MATERIAL", Name: "Material", Active: true}},
		Families:    []ResourceFamily{{ClassCode: "MATERIAL", Code: "CONDUCTORES", Name: "Conductores", Active: true}},
		Types:       []ResourceType{{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Code: "CABLE", Name: "Cable", Active: true}},
		Definitions: []AttributeDefinition{{Code: "color", Name: "Color", ValueType: ValueTypeControlledOption, Active: true}, {Code: "size", Name: "Tamaño", ValueType: ValueTypeControlledOption, Active: true}},
		OptionSets:  []ResourceOptionSet{{Code: "COLORS", Name: "Colores", Active: true}},
	}
}

func remainingRef(kind CatalogKindCode, code string) CatalogValue {
	return CatalogValue{Ref: CatalogRef{Kind: kind, Code: code}}
}

func remainingLength(c ResourceCatalog, kind CatalogKindCode) int {
	switch kind {
	case KindOption:
		return len(c.Options)
	case KindOptionRelation:
		return len(c.Relations)
	case KindUnit:
		return len(c.Units)
	case KindUnitPolicy:
		return len(c.UnitPolicies)
	case KindAttributeBinding:
		return len(c.Attributes)
	case KindPresentationField:
		return len(c.PresentationFields)
	default:
		return -1
	}
}

func remainingLast(c ResourceCatalog, kind CatalogKindCode) interface{} {
	switch kind {
	case KindOption:
		return c.Options[len(c.Options)-1]
	case KindOptionRelation:
		return c.Relations[len(c.Relations)-1]
	case KindUnit:
		return c.Units[len(c.Units)-1]
	case KindUnitPolicy:
		return c.UnitPolicies[len(c.UnitPolicies)-1]
	case KindAttributeBinding:
		return c.Attributes[len(c.Attributes)-1]
	case KindPresentationField:
		return c.PresentationFields[len(c.PresentationFields)-1]
	default:
		return nil
	}
}

func remainingWant(kind CatalogKindCode, r CatalogRecord, active bool) interface{} {
	switch kind {
	case KindOption:
		return AttributeOption{OptionSet: ref(r, "optionSet"), AttributeCode: ref(r, "characteristic"), Code: text(r, "code"), Label: text(r, "label"), Active: active}
	case KindOptionRelation:
		return AttributeOptionRelation{OptionSet: ref(r, "optionSet"), FromAttribute: ref(r, "fromCharacteristic"), FromOption: ref(r, "fromOption"), ToAttribute: ref(r, "toCharacteristic"), ToOption: ref(r, "toOption"), Active: active}
	case KindUnit:
		return UnitDefinition{Code: text(r, "code"), Name: text(r, "name"), Symbol: text(r, "symbol"), Dimension: text(r, "dimension"), Active: active}
	case KindUnitPolicy:
		return ResourceUnitPolicy{ClassCode: ref(r, "class"), FamilyCode: ref(r, "family"), UnitCode: ref(r, "unit"), Allowed: boolean(r, "allowed"), Suggested: boolean(r, "suggested"), Active: active}
	case KindPresentationField:
		return PresentationField{ClassCode: ref(r, "class"), FamilyCode: ref(r, "family"), TypeCode: ref(r, "type"), AttributeCode: ref(r, "characteristic"), Position: integer(r, "position"), Active: active}
	}
	attribute := ResourceAttribute{ClassCode: ref(r, "class"), FamilyCode: ref(r, "family"), TypeCode: ref(r, "type"), OptionSet: ref(r, "optionSet"), Mode: AttributeMode(text(r, "mode")), IdentityParticipates: boolean(r, "identityParticipates"), Active: active}
	attribute.Definition = remainingKindCatalog().Definitions[0]
	attribute.Rules = attributeRulesFromRecords(r.Rules)
	return attribute
}

func TestApplyCatalogMutation_RemainingKindLosslessLifecycleMatrix(t *testing.T) {
	conditionalRules := []CatalogRuleRecord{{When: AttributeCondition{AttributeCode: "size", Equals: "LARGE"}, Mode: ModeRequired, IdentityParticipates: true, Active: true}, {When: AttributeCondition{AttributeCode: "size", Equals: "SMALL"}, Mode: ModeOptional, Active: false}}
	cases := []struct {
		name                string
		kind                CatalogKindCode
		insert, replacement CatalogRecord
	}{
		{string(KindOption), KindOption, CatalogRecord{Kind: KindOption, Active: true, Values: map[string]CatalogValue{"optionSet": remainingRef(KindOptionSet, "COLORS"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "code": {Text: "RED"}, "label": {Text: "Rojo"}}}, CatalogRecord{Kind: KindOption, Values: map[string]CatalogValue{"optionSet": remainingRef(KindOptionSet, "COLORS"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "code": {Text: "RED"}, "label": {Text: "Rojo nuevo"}}}},
		{string(KindOptionRelation), KindOptionRelation, CatalogRecord{Kind: KindOptionRelation, Active: true, Values: map[string]CatalogValue{"optionSet": remainingRef(KindOptionSet, "COLORS"), "fromCharacteristic": remainingRef(KindAttributeDefinition, "color"), "fromOption": remainingRef(KindOption, "RED"), "toCharacteristic": remainingRef(KindAttributeDefinition, "size"), "toOption": remainingRef(KindOption, "LARGE")}}, CatalogRecord{Kind: KindOptionRelation, Values: map[string]CatalogValue{"optionSet": remainingRef(KindOptionSet, "COLORS"), "fromCharacteristic": remainingRef(KindAttributeDefinition, "color"), "fromOption": remainingRef(KindOption, "RED"), "toCharacteristic": remainingRef(KindAttributeDefinition, "size"), "toOption": remainingRef(KindOption, "LARGE")}}},
		{string(KindUnit), KindUnit, CatalogRecord{Kind: KindUnit, Active: true, Values: map[string]CatalogValue{"code": {Text: "M"}, "name": {Text: "Metro"}, "symbol": {Text: "m"}, "dimension": {Text: "LENGTH"}}}, CatalogRecord{Kind: KindUnit, Values: map[string]CatalogValue{"code": {Text: "M"}, "name": {Text: "Metro lineal"}, "symbol": {Text: "m"}, "dimension": {Text: "LENGTH"}}}},
		{string(KindUnitPolicy), KindUnitPolicy, CatalogRecord{Kind: KindUnitPolicy, Active: true, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "unit": remainingRef(KindUnit, "M"), "allowed": {Bool: true}, "suggested": {Bool: false}}}, CatalogRecord{Kind: KindUnitPolicy, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "unit": remainingRef(KindUnit, "M"), "allowed": {Bool: false}, "suggested": {Bool: true}}}},
		{string(KindAttributeBinding), KindAttributeBinding, CatalogRecord{Kind: KindAttributeBinding, Active: true, Rules: conditionalRules, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "type": remainingRef(KindType, "CABLE"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "optionSet": remainingRef(KindOptionSet, "COLORS"), "mode": {Text: string(ModeConditional)}, "identityParticipates": {Bool: true}}}, CatalogRecord{Kind: KindAttributeBinding, Rules: conditionalRules, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "type": remainingRef(KindType, "CABLE"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "optionSet": remainingRef(KindOptionSet, "COLORS"), "mode": {Text: string(ModeConditional)}}}},
		{string(KindPresentationField), KindPresentationField, CatalogRecord{Kind: KindPresentationField, Active: true, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "type": remainingRef(KindType, "CABLE"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "position": {Int: 3}}}, CatalogRecord{Kind: KindPresentationField, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "type": remainingRef(KindType, "CABLE"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "position": {Int: 7}}}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			original, registry := remainingKindCatalog(), NewCatalogRegistry()
			next, err := ApplyCatalogMutation(original, registry, CatalogMutation{Op: OpInsert, Record: tt.insert})
			if err != nil {
				t.Fatalf("insert: %v", err)
			}
			if got, want := remainingLast(next, tt.kind), remainingWant(tt.kind, tt.insert, true); !reflect.DeepEqual(got, want) {
				t.Fatalf("insert = %#v, want %#v", got, want)
			}
			updated, err := ApplyCatalogMutation(next, registry, CatalogMutation{Op: OpUpdate, Record: tt.insert, Replacement: &tt.replacement})
			if err != nil {
				t.Fatalf("update: %v", err)
			}
			if got, want := remainingLast(updated, tt.kind), remainingWant(tt.kind, tt.replacement, false); !reflect.DeepEqual(got, want) {
				t.Fatalf("update = %#v, want %#v", got, want)
			}
			inactive, err := ApplyCatalogMutation(updated, registry, CatalogMutation{Op: OpDeactivate, Record: tt.insert})
			if err != nil {
				t.Fatalf("deactivate: %v", err)
			}
			active, err := ApplyCatalogMutation(inactive, registry, CatalogMutation{Op: OpReactivate, Record: tt.insert})
			if err != nil {
				t.Fatalf("reactivate: %v", err)
			}
			if got, want := remainingLast(active, tt.kind), remainingWant(tt.kind, tt.replacement, true); !reflect.DeepEqual(got, want) {
				t.Fatalf("reactivate = %#v, want %#v", got, want)
			}
			deleted, err := ApplyCatalogMutation(active, registry, CatalogMutation{Op: OpDelete, Record: tt.insert})
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			if remainingLength(deleted, tt.kind) != 0 || remainingLength(original, tt.kind) != 0 {
				t.Fatalf("delete changed candidate or prior snapshot: %#v / %#v", deleted, original)
			}
		})
	}
}

func TestApplyCatalogMutation_DefinitionUpdateRebuildsAllEmbeddedDefinitions(t *testing.T) {
	catalog := remainingKindCatalog()
	catalog.Attributes = []ResourceAttribute{{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Definition: catalog.Definitions[0], Active: true}, {ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", Definition: catalog.Definitions[0], Active: false}}
	replacement := CatalogRecord{Kind: KindAttributeDefinition, Values: map[string]CatalogValue{"code": {Text: "color"}, "name": {Text: "Tono"}, "valueType": {Text: string(ValueTypeControlledText)}, "dimension": {Text: "TEXT"}}}
	next, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpUpdate, Record: CatalogRecord{Kind: KindAttributeDefinition, Values: map[string]CatalogValue{"code": {Text: "color"}}}, Replacement: &replacement})
	if err != nil {
		t.Fatalf("definition update: %v", err)
	}
	want := AttributeDefinition{Code: "color", Name: "Tono", ValueType: ValueTypeControlledText, Dimension: "TEXT", Active: false}
	if next.Definitions[0] != want || next.Attributes[0].Definition != want || next.Attributes[1].Definition != want {
		t.Fatalf("definition rebuild lost fields: %#v", next)
	}
	if catalog.Definitions[0].Name != "Color" || catalog.Attributes[0].Definition.Name != "Color" {
		t.Fatal("definition update changed prior snapshot")
	}
	rename := replacement
	rename.Values = map[string]CatalogValue{"code": {Text: "colour"}, "name": {Text: "Tono"}}
	if _, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpUpdate, Record: CatalogRecord{Kind: KindAttributeDefinition, Values: map[string]CatalogValue{"code": {Text: "color"}}}, Replacement: &rename}); !errors.Is(err, ErrCodeImmutable) {
		t.Fatalf("referenced natural-code replacement error = %v, want ErrCodeImmutable", err)
	}
}

func TestApplyCatalogMutation_ApplicabilityRejectsIncompleteAggregate(t *testing.T) {
	catalog := remainingKindCatalog()
	catalog.Attributes = []ResourceAttribute{{ClassCode: "MATERIAL", FamilyCode: "CONDUCTORES", TypeCode: "CABLE", Definition: catalog.Definitions[0], Mode: ModeRequired, Active: true}}
	base := CatalogRecord{Kind: KindAttributeBinding, Active: true, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "type": remainingRef(KindType, "CABLE"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "mode": {Text: string(ModeRequired)}}}
	cases := []struct {
		name   string
		record CatalogRecord
		want   error
	}{
		{"omitted rules", base, ErrResourceValidation},
		{"unknown characteristic", func() CatalogRecord {
			r := cloneCatalogRecord(base)
			r.Rules = []CatalogRuleRecord{}
			r.Values["characteristic"] = remainingRef(KindAttributeDefinition, "missing")
			return r
		}(), ErrResourceReference},
		{"incomplete rule", func() CatalogRecord {
			r := cloneCatalogRecord(base)
			r.Rules = []CatalogRuleRecord{{Mode: ModeRequired}}
			return r
		}(), ErrResourceValidation},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			before := cloneMutationCatalog(catalog)
			_, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpInsert, Record: tt.record})
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(catalog, before) {
				t.Fatal("invalid aggregate changed prior snapshot")
			}
		})
	}
}

func TestApplyCatalogMutation_ApplicabilityPreservesExplicitEmptyAndCallerIsolation(t *testing.T) {
	catalog := remainingKindCatalog()
	record := CatalogRecord{Kind: KindAttributeBinding, Rules: []CatalogRuleRecord{}, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "mode": {Text: string(ModeRequired)}}}
	next, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpInsert, Record: record})
	if err != nil {
		t.Fatalf("explicit empty rules: %v", err)
	}
	if next.Attributes[0].Rules == nil {
		t.Fatal("explicit empty rules became omitted")
	}
	record.Values["mode"] = CatalogValue{Text: string(ModeConditional)}
	record.Rules = []CatalogRuleRecord{{When: AttributeCondition{AttributeCode: "size", Equals: "LARGE"}, Mode: ModeRequired}, {When: AttributeCondition{AttributeCode: "size", Equals: "SMALL"}, Mode: ModeOptional}}
	candidate, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpInsert, Record: record})
	if err != nil || len(candidate.Attributes) != 1 || candidate.Attributes[0].Rules[0].When.Equals != "LARGE" || candidate.Attributes[0].Rules[1].When.Equals != "SMALL" {
		t.Fatalf("ordered rules = %#v, error %v", candidate.Attributes, err)
	}
	reordered := cloneCatalogRecord(record)
	reordered.Rules = []CatalogRuleRecord{record.Rules[1], record.Rules[0]}
	other, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpInsert, Record: reordered})
	if err != nil || reflect.DeepEqual(candidate.Attributes[0].Rules, other.Attributes[0].Rules) {
		t.Fatalf("reordered rules were treated as equal: %#v / %#v", candidate.Attributes[0].Rules, other.Attributes[0].Rules)
	}
	record.Rules[0].When.Equals = "caller changed"
	candidate.Attributes[0].Rules[0].When.Equals = "candidate changed"
	if catalog.Attributes != nil || candidate.Attributes[0].Rules[0].When.Equals != "candidate changed" {
		t.Fatal("aggregate caller/prior snapshot isolation failed")
	}
}

func TestCatalogMutation_DefinitionLifecycleSynchronizesEmbeddedActiveState(t *testing.T) {
	catalog := ResourceCatalog{
		Definitions: []AttributeDefinition{{Code: "color", Name: "Color", Active: true}},
		Attributes: []ResourceAttribute{
			{Definition: AttributeDefinition{Code: "color", Name: "Color", Active: true}, Active: true},
			{Definition: AttributeDefinition{Code: "color", Name: "Color", Active: true}, Active: false},
		},
	}
	record := CatalogRecord{Kind: KindAttributeDefinition, Values: map[string]CatalogValue{"code": {Text: "color"}}}

	inactive, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpDeactivate, Record: record})
	if err != nil {
		t.Fatalf("deactivate definition: %v", err)
	}
	if inactive.Definitions[0].Active || inactive.Attributes[0].Definition.Active || inactive.Attributes[1].Definition.Active {
		t.Fatalf("deactivation did not synchronize embedded definition state: %#v", inactive)
	}
	if !inactive.Attributes[0].Active || inactive.Attributes[1].Active || inactive.Attributes[0].Definition.Name != "Color" || inactive.Attributes[1].Definition.Name != "Color" {
		t.Fatalf("deactivation changed binding or definition data: %#v", inactive.Attributes)
	}

	active, err := ApplyCatalogMutation(inactive, NewCatalogRegistry(), CatalogMutation{Op: OpReactivate, Record: record})
	if err != nil {
		t.Fatalf("reactivate definition: %v", err)
	}
	if !active.Definitions[0].Active || !active.Attributes[0].Definition.Active || !active.Attributes[1].Definition.Active {
		t.Fatalf("reactivation did not synchronize embedded definition state: %#v", active)
	}
	if !active.Attributes[0].Active || active.Attributes[1].Active || active.Attributes[0].Definition.Name != "Color" || active.Attributes[1].Definition.Name != "Color" {
		t.Fatalf("reactivation changed binding or definition data: %#v", active.Attributes)
	}
}

func TestCatalogMutation_ApplicabilityLifecyclePreservesNestedRuleActiveValues(t *testing.T) {
	catalog := remainingKindCatalog()
	record := CatalogRecord{
		Kind:   KindAttributeBinding,
		Active: true,
		Rules: []CatalogRuleRecord{
			{When: AttributeCondition{AttributeCode: "size", Equals: "LARGE"}, Mode: ModeRequired, Active: true},
			{When: AttributeCondition{AttributeCode: "size", Equals: "SMALL"}, Mode: ModeOptional, Active: false},
		},
		Values: map[string]CatalogValue{
			"class":          {Ref: CatalogRef{Kind: KindClass, Code: "MATERIAL"}},
			"family":         {Ref: CatalogRef{Kind: KindFamily, Code: "CONDUCTORES"}},
			"type":           {Ref: CatalogRef{Kind: KindType, Code: "CABLE"}},
			"characteristic": {Ref: CatalogRef{Kind: KindAttributeDefinition, Code: "color"}},
			"optionSet":      {Ref: CatalogRef{Kind: KindOptionSet, Code: "COLORS"}},
			"mode":           {Text: string(ModeConditional)},
		},
	}
	inserted, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpInsert, Record: record})
	if err != nil {
		t.Fatalf("insert applicability: %v", err)
	}
	if !inserted.Attributes[0].Active || !inserted.Attributes[0].Rules[0].Active || inserted.Attributes[0].Rules[1].Active {
		t.Fatalf("insert lost active states: %#v", inserted.Attributes[0])
	}

	replacement := cloneCatalogRecord(record)
	replacement.Active = false
	replacement.Rules[0].Active = false
	replacement.Rules[1].Active = true
	updated, err := ApplyCatalogMutation(inserted, NewCatalogRegistry(), CatalogMutation{Op: OpUpdate, Record: record, Replacement: &replacement})
	if err != nil {
		t.Fatalf("update applicability: %v", err)
	}
	if updated.Attributes[0].Active || updated.Attributes[0].Rules[0].Active || !updated.Attributes[0].Rules[1].Active {
		t.Fatalf("update lost parent or nested active states: %#v", updated.Attributes[0])
	}

	deactivated, err := ApplyCatalogMutation(updated, NewCatalogRegistry(), CatalogMutation{Op: OpDeactivate, Record: record})
	if err != nil {
		t.Fatalf("deactivate applicability: %v", err)
	}
	if deactivated.Attributes[0].Active || deactivated.Attributes[0].Rules[0].Active || !deactivated.Attributes[0].Rules[1].Active {
		t.Fatalf("deactivation changed nested rule states: %#v", deactivated.Attributes[0])
	}
	active, err := ApplyCatalogMutation(deactivated, NewCatalogRegistry(), CatalogMutation{Op: OpReactivate, Record: record})
	if err != nil {
		t.Fatalf("reactivate applicability: %v", err)
	}
	if !active.Attributes[0].Active || active.Attributes[0].Rules[0].Active || !active.Attributes[0].Rules[1].Active {
		t.Fatalf("reactivation changed nested rule states: %#v", active.Attributes[0])
	}
	replacement.Rules[0].Active = true
	if updated.Attributes[0].Rules[0].Active {
		t.Fatal("updated applicability shares nested rule storage with replacement")
	}
	if !inserted.Attributes[0].Active || !inserted.Attributes[0].Rules[0].Active || inserted.Attributes[0].Rules[1].Active {
		t.Fatal("applicability lifecycle changed the prior snapshot")
	}
}

func TestCatalogMutation_AllKindsSupportLifecycleOperations(t *testing.T) {
	records := []CatalogRecord{
		{Kind: KindClass, Values: map[string]CatalogValue{"code": {Text: "TOOLS"}, "name": {Text: "Tools"}, "plural": {Text: "Tools"}, "slug": {Text: "tools"}}},
		{Kind: KindFamily, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "code": {Text: "TOOLS"}, "name": {Text: "Tools"}}},
		{Kind: KindType, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "code": {Text: "TOOL"}, "name": {Text: "Tool"}}},
		{Kind: KindAttributeDefinition, Values: map[string]CatalogValue{"code": {Text: "finish"}, "name": {Text: "Finish"}, "valueType": {Text: string(ValueTypeControlledText)}}},
		{Kind: KindOptionSet, Values: map[string]CatalogValue{"code": {Text: "FINISHES"}, "name": {Text: "Finishes"}}},
		{Kind: KindOption, Values: map[string]CatalogValue{"optionSet": remainingRef(KindOptionSet, "COLORS"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "code": {Text: "BLUE"}, "label": {Text: "Blue"}}},
		{Kind: KindOptionRelation, Values: map[string]CatalogValue{"optionSet": remainingRef(KindOptionSet, "COLORS"), "fromCharacteristic": remainingRef(KindAttributeDefinition, "color"), "fromOption": remainingRef(KindOption, "RED"), "toCharacteristic": remainingRef(KindAttributeDefinition, "size"), "toOption": remainingRef(KindOption, "LARGE")}},
		{Kind: KindUnit, Values: map[string]CatalogValue{"code": {Text: "CM"}, "name": {Text: "Centimeter"}, "symbol": {Text: "cm"}, "dimension": {Text: "LENGTH"}}},
		{Kind: KindUnitPolicy, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "unit": remainingRef(KindUnit, "M"), "allowed": {Bool: true}, "suggested": {Bool: false}}},
		{Kind: KindAttributeBinding, Rules: []CatalogRuleRecord{}, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "type": remainingRef(KindType, "CABLE"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "mode": {Text: string(ModeRequired)}}},
		{Kind: KindPresentationField, Values: map[string]CatalogValue{"class": remainingRef(KindClass, "MATERIAL"), "family": remainingRef(KindFamily, "CONDUCTORES"), "type": remainingRef(KindType, "CABLE"), "characteristic": remainingRef(KindAttributeDefinition, "color"), "position": {Int: 1}}},
	}
	for _, record := range records {
		t.Run(string(record.Kind), func(t *testing.T) {
			catalog, registry := remainingKindCatalog(), NewCatalogRegistry()
			base := allKindCollectionLength(catalog, record.Kind)
			record.Active = true
			inserted, err := ApplyCatalogMutation(catalog, registry, CatalogMutation{Op: OpInsert, Record: record})
			if err != nil {
				t.Fatalf("insert: %v", err)
			}
			if got := allKindCollectionLength(inserted, record.Kind); got != base+1 {
				t.Fatalf("insert count = %d, want %d", got, base+1)
			}

			replacement := cloneCatalogRecord(record)
			replacement.Active = false
			updated, err := ApplyCatalogMutation(inserted, registry, CatalogMutation{Op: OpUpdate, Record: record, Replacement: &replacement})
			if err != nil {
				t.Fatalf("update: %v", err)
			}
			deactivated, err := ApplyCatalogMutation(updated, registry, CatalogMutation{Op: OpDeactivate, Record: record})
			if err != nil {
				t.Fatalf("deactivate: %v", err)
			}
			if allKindActive(deactivated, record.Kind) {
				t.Fatalf("deactivate left %s active", record.Kind)
			}
			reactivated, err := ApplyCatalogMutation(deactivated, registry, CatalogMutation{Op: OpReactivate, Record: record})
			if err != nil {
				t.Fatalf("reactivate: %v", err)
			}
			if !allKindActive(reactivated, record.Kind) {
				t.Fatalf("reactivate left %s inactive", record.Kind)
			}
			reactivatedAgain, err := ApplyCatalogMutation(reactivated, registry, CatalogMutation{Op: OpReactivate, Record: record})
			if err != nil || !allKindActive(reactivatedAgain, record.Kind) {
				t.Fatalf("idempotent reactivate: %v", err)
			}
			deleted, err := ApplyCatalogMutation(reactivatedAgain, registry, CatalogMutation{Op: OpDelete, Record: record})
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			if got := allKindCollectionLength(deleted, record.Kind); got != base {
				t.Fatalf("delete count = %d, want original count %d", got, base)
			}
		})
	}
}

func allKindCollectionLength(catalog ResourceCatalog, kind CatalogKindCode) int {
	switch kind {
	case KindClass:
		return len(catalog.Classes)
	case KindFamily:
		return len(catalog.Families)
	case KindType:
		return len(catalog.Types)
	case KindAttributeDefinition:
		return len(catalog.Definitions)
	case KindOptionSet:
		return len(catalog.OptionSets)
	case KindOption:
		return len(catalog.Options)
	case KindOptionRelation:
		return len(catalog.Relations)
	case KindUnit:
		return len(catalog.Units)
	case KindUnitPolicy:
		return len(catalog.UnitPolicies)
	case KindAttributeBinding:
		return len(catalog.Attributes)
	case KindPresentationField:
		return len(catalog.PresentationFields)
	default:
		return -1
	}
}

func allKindActive(catalog ResourceCatalog, kind CatalogKindCode) bool {
	switch kind {
	case KindClass:
		return catalog.Classes[len(catalog.Classes)-1].Active
	case KindFamily:
		return catalog.Families[len(catalog.Families)-1].Active
	case KindType:
		return catalog.Types[len(catalog.Types)-1].Active
	case KindAttributeDefinition:
		return catalog.Definitions[len(catalog.Definitions)-1].Active
	case KindOptionSet:
		return catalog.OptionSets[len(catalog.OptionSets)-1].Active
	case KindOption:
		return catalog.Options[len(catalog.Options)-1].Active
	case KindOptionRelation:
		return catalog.Relations[len(catalog.Relations)-1].Active
	case KindUnit:
		return catalog.Units[len(catalog.Units)-1].Active
	case KindUnitPolicy:
		return catalog.UnitPolicies[len(catalog.UnitPolicies)-1].Active
	case KindAttributeBinding:
		return catalog.Attributes[len(catalog.Attributes)-1].Active
	case KindPresentationField:
		return catalog.PresentationFields[len(catalog.PresentationFields)-1].Active
	default:
		return false
	}
}

func TestCatalogMutation_MissingDefinitionLifecycleIsNotUnsupported(t *testing.T) {
	_, err := ApplyCatalogMutation(ResourceCatalog{}, NewCatalogRegistry(), CatalogMutation{
		Op: OpDeactivate,
		Record: CatalogRecord{Kind: KindAttributeDefinition, Values: map[string]CatalogValue{
			"code": {Text: "missing"},
		}},
	})
	if !errors.Is(err, ErrCatalogRecordNotFound) {
		t.Fatalf("missing definition lifecycle error = %v, want ErrCatalogRecordNotFound", err)
	}
	if strings.Contains(err.Error(), "does not support deactivate/reactivate") {
		t.Fatalf("missing definition lifecycle retained unsupported behavior: %v", err)
	}
}

func TestCatalogMutation_UnknownKindAndOperationRemainExplicitErrors(t *testing.T) {
	_, err := ApplyCatalogMutation(ResourceCatalog{}, NewCatalogRegistry(), CatalogMutation{
		Op: OpInsert, Record: CatalogRecord{Kind: CatalogKindCode("NOPE")},
	})
	if !errors.Is(err, ErrCatalogKindUnknown) {
		t.Fatalf("unknown kind error = %v, want ErrCatalogKindUnknown", err)
	}
	_, err = ApplyCatalogMutation(ResourceCatalog{}, NewCatalogRegistry(), CatalogMutation{
		Op: MutationOp(99), Record: CatalogRecord{Kind: KindClass},
	})
	if !errors.Is(err, ErrMutationOpUnknown) {
		t.Fatalf("unknown operation error = %v, want ErrMutationOpUnknown", err)
	}
}
