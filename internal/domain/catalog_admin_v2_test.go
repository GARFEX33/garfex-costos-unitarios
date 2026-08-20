package domain

import (
	"context"
	"errors"
	"testing"
)

var errFakeCatalogAdminV2StaleRevision = errors.New("fake catalog admin v2: stale expected revision")

// fakeCatalogAdminRepositoryV2 is a minimal in-memory double proving
// CatalogAdminRepositoryV2 is complete and satisfiable. Test double only.
type fakeCatalogAdminRepositoryV2 struct {
	nextID  int64
	records map[int64]CatalogRecord
	catalog ResourceCatalog
}

var _ CatalogAdminRepositoryV2 = (*fakeCatalogAdminRepositoryV2)(nil)

func newFakeCatalogAdminRepositoryV2() *fakeCatalogAdminRepositoryV2 {
	return &fakeCatalogAdminRepositoryV2{records: map[int64]CatalogRecord{}}
}

func (f *fakeCatalogAdminRepositoryV2) Insert(_ context.Context, rec CatalogRecord) (CatalogWriteResult, error) {
	f.nextID++
	rec.ID, rec.Revision = f.nextID, 1
	stored := cloneCatalogRecord(rec)
	f.records[stored.ID] = stored
	f.catalog.Classes = append(f.catalog.Classes, ResourceClass{
		Code: stored.Values["code"].Text, Aliases: cloneCatalogStrings(stored.Values["code"].List), Active: true,
	})
	return NewCatalogWriteResult(&stored, f.catalog), nil
}

// casCurrent is the shared CAS check for Update/SetActive/Delete.
func (f *fakeCatalogAdminRepositoryV2) casCurrent(id int64, expectedRevision uint64) (CatalogRecord, error) {
	current, ok := f.records[id]
	if !ok {
		return CatalogRecord{}, errors.New("fake catalog admin v2: unknown record")
	}
	if expectedRevision == 0 || expectedRevision != current.Revision {
		return CatalogRecord{}, errFakeCatalogAdminV2StaleRevision
	}
	return current, nil
}

func (f *fakeCatalogAdminRepositoryV2) Update(_ context.Context, rec CatalogRecord, expectedRevision uint64) (CatalogWriteResult, error) {
	current, err := f.casCurrent(rec.ID, expectedRevision)
	if err != nil {
		return CatalogWriteResult{}, err
	}
	rec.Revision = current.Revision + 1
	stored := cloneCatalogRecord(rec)
	f.records[stored.ID] = stored
	return NewCatalogWriteResult(&stored, f.catalog), nil
}

func (f *fakeCatalogAdminRepositoryV2) SetActive(_ context.Context, kind CatalogKindCode, id int64, active bool, expectedRevision uint64) (CatalogWriteResult, error) {
	current, err := f.casCurrent(id, expectedRevision)
	if err != nil {
		return CatalogWriteResult{}, err
	}
	if current.Active != active {
		current.Revision++
	}
	current.Active, current.Kind = active, kind
	f.records[id] = current
	return NewCatalogWriteResult(&current, f.catalog), nil
}

func (f *fakeCatalogAdminRepositoryV2) Delete(_ context.Context, _ CatalogKindCode, id int64, expectedRevision uint64) (CatalogWriteResult, error) {
	if _, err := f.casCurrent(id, expectedRevision); err != nil {
		return CatalogWriteResult{}, err
	}
	delete(f.records, id)
	return NewCatalogWriteResult(nil, f.catalog), nil
}

// legacyFakeCatalogAdminRepository proves CatalogAdminRepository still
// compiles completely unchanged alongside the new additive V2 one:
// embedding the interface promotes its exact method set, so any signature
// drift in catalog_record.go fails this compile-time assertion.
type legacyFakeCatalogAdminRepository struct{ CatalogAdminRepository }

var _ CatalogAdminRepository = legacyFakeCatalogAdminRepository{}

func TestCatalogWriteResultConstructorCopiesRecordAndCatalog(t *testing.T) {
	record := CatalogRecord{Kind: KindClass, ID: 1, Revision: 1, Values: map[string]CatalogValue{
		"code": {Text: "ORIGINAL", List: []string{"alias-1"}},
	}}
	catalog := ResourceCatalog{Classes: []ResourceClass{{Code: "ORIGINAL", Aliases: []string{"alias-1"}, Active: true}}}
	result := NewCatalogWriteResult(&record, catalog)

	mutatedCode := record.Values["code"]
	mutatedCode.Text = "CALLER MUTATED"
	mutatedCode.List[0] = "caller mutated alias"
	record.Values["code"] = mutatedCode
	catalog.Classes[0].Code = "CALLER MUTATED"
	catalog.Classes[0].Aliases[0] = "caller mutated alias"

	if result.Record.Values["code"].Text != "ORIGINAL" || result.Record.Values["code"].List[0] != "alias-1" {
		t.Fatalf("result.Record changed through caller storage: %#v", result.Record.Values["code"])
	}
	if result.Catalog.Classes[0].Code != "ORIGINAL" || result.Catalog.Classes[0].Aliases[0] != "alias-1" {
		t.Fatalf("result.Catalog changed through caller storage: %#v", result.Catalog.Classes[0])
	}
}

func TestCatalogWriteResultConstructorNilRecordStaysNil(t *testing.T) {
	if got := NewCatalogWriteResult(nil, ResourceCatalog{}); got.Record != nil {
		t.Fatalf("Record = %#v, want nil", got.Record)
	}
}

func TestFakeCatalogAdminRepositoryV2InsertResultIsolatedFromStorage(t *testing.T) {
	fake := newFakeCatalogAdminRepositoryV2()
	rec := CatalogRecord{Kind: KindClass, Values: map[string]CatalogValue{"code": {Text: "MATERIAL", List: []string{"alias-1"}}}}
	result, err := fake.Insert(context.Background(), rec)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Caller-side mutation of the returned record must never reach storage.
	result.Record.Values["code"] = CatalogValue{Text: "MUTATED"}
	if stored := fake.records[result.Record.ID]; stored.Values["code"].Text != "MATERIAL" {
		t.Fatalf("caller mutation leaked into record storage: %#v", stored.Values["code"])
	}

	// Repository-side mutation after the call must never reach the
	// already-returned result.
	fake.catalog.Classes[0].Aliases[0] = "repository-side mutation"
	if result.Catalog.Classes[0].Aliases[0] != "alias-1" {
		t.Fatalf("repository-side mutation leaked into a prior result: %#v", result.Catalog.Classes[0])
	}
}

func TestFakeCatalogAdminRepositoryV2UpdateRejectsStaleRevision(t *testing.T) {
	fake := newFakeCatalogAdminRepositoryV2()
	rec := CatalogRecord{Kind: KindClass, Values: map[string]CatalogValue{"code": {Text: "MATERIAL"}}}
	inserted, err := fake.Insert(context.Background(), rec)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if _, err := fake.Update(context.Background(), *inserted.Record, 0); err == nil {
		t.Fatal("Update() with expectedRevision 0 succeeded, want an error")
	}
	if _, err := fake.Update(context.Background(), *inserted.Record, inserted.Record.Revision+1); !errors.Is(err, errFakeCatalogAdminV2StaleRevision) {
		t.Fatalf("Update() with stale expectedRevision error = %v, want %v", err, errFakeCatalogAdminV2StaleRevision)
	}

	result, err := fake.Update(context.Background(), *inserted.Record, inserted.Record.Revision)
	if err != nil {
		t.Fatalf("Update() with current expectedRevision error = %v", err)
	}
	if result.Record.Revision != inserted.Record.Revision+1 {
		t.Fatalf("result.Record.Revision = %d, want %d", result.Record.Revision, inserted.Record.Revision+1)
	}
}

func TestFakeCatalogAdminRepositoryV2DeleteReturnsNilRecord(t *testing.T) {
	fake := newFakeCatalogAdminRepositoryV2()
	inserted, err := fake.Insert(context.Background(), CatalogRecord{Kind: KindClass, Values: map[string]CatalogValue{"code": {Text: "MATERIAL"}}})
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	result, err := fake.Delete(context.Background(), KindClass, inserted.Record.ID, inserted.Record.Revision)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if result.Record != nil {
		t.Fatalf("Delete() result.Record = %#v, want nil", result.Record)
	}
	if _, ok := fake.records[inserted.Record.ID]; ok {
		t.Fatal("Delete() left the record in repository storage")
	}
}
