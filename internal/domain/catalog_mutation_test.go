package domain

import "testing"

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

// TestApplyCatalogMutation_DeactivateUnsupportedKindReturnsError proves a
// kind whose ResourceCatalog struct has no Active field yet (design
// CatalogKind.SoftDelete == false, e.g. AttributeDefinition) rejects
// Deactivate/Reactivate explicitly instead of silently no-op'ing.
func TestApplyCatalogMutation_DeactivateUnsupportedKindReturnsError(t *testing.T) {
	catalog := ResourceCatalog{Definitions: []AttributeDefinition{{Code: "color", Name: "Color"}}}
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpDeactivate,
		Record: CatalogRecord{
			Kind:   KindAttributeDefinition,
			Values: map[string]CatalogValue{"code": {Text: "color"}},
		},
	}

	if _, err := ApplyCatalogMutation(catalog, registry, mutation); err == nil {
		t.Fatal("ApplyCatalogMutation(Deactivate, AttributeDefinition) error = nil, want an error (no Active field on this Go struct yet)")
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

// TestApplyCatalogMutation_OptionSetIsANoOpOnTheSnapshot proves KindOptionSet
// — which has no ResourceCatalog slice representation (OptionSet is a
// denormalized string tag on Options/Attributes/Relations, not its own
// entity in the pure domain snapshot) — is a documented no-op: the
// repository (a later PR) owns its persistence independently.
func TestApplyCatalogMutation_OptionSetIsANoOpOnTheSnapshot(t *testing.T) {
	catalog := baseMutationCatalog()
	registry := NewCatalogRegistry()
	mutation := CatalogMutation{
		Op: OpInsert,
		Record: CatalogRecord{
			Kind:   KindOptionSet,
			Values: map[string]CatalogValue{"code": {Text: "COLORES"}, "name": {Text: "Colores"}},
		},
	}

	next, err := ApplyCatalogMutation(catalog, registry, mutation)
	if err != nil {
		t.Fatalf("ApplyCatalogMutation(OptionSet) error = %v, want nil", err)
	}
	if len(next.Families) != len(catalog.Families) || len(next.Classes) != len(catalog.Classes) {
		t.Fatalf("ApplyCatalogMutation(OptionSet) changed unrelated slices, want the snapshot unchanged")
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
