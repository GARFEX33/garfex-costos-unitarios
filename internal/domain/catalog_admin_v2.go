package domain

import "context"

// CatalogWriteResult is the additive end-state result of one
// CatalogAdminRepositoryV2 mutation (design "Catalog transaction and
// coherent publication"): Record is the record the mutation produced or
// touched (nil when there is none — currently only Delete), and Catalog is
// the complete coherent ResourceCatalog the adapter reloaded and validated
// in the same transaction after commit — never a hand-built candidate.
// Both fields are defensively copied by NewCatalogWriteResult so neither a
// later caller mutation nor a later repository mutation crosses over.
type CatalogWriteResult struct {
	Record  *CatalogRecord
	Catalog ResourceCatalog
}

// NewCatalogWriteResult builds a CatalogWriteResult, defensively copying
// record and catalog so the caller and the adapter never share backing
// storage. A CatalogAdminRepositoryV2 implementation MUST use this
// constructor, or apply the same copying discipline, rather than returning
// caller- or adapter-owned storage directly.
func NewCatalogWriteResult(record *CatalogRecord, catalog ResourceCatalog) CatalogWriteResult {
	result := CatalogWriteResult{Catalog: cloneMutationCatalog(catalog)}
	if record != nil {
		copyOfRecord := cloneCatalogRecord(*record)
		result.Record = &copyOfRecord
	}
	return result
}

// CatalogAdminRepositoryV2 is the end-state, revision-aware catalog admin
// write port (design "Catalog transaction and coherent publication"),
// additive and currently dormant — no production composition root,
// service, or PostgreSQL constructor uses it yet. Stage 3D defines only
// this domain contract and its fakes; stages 3E-3G build the PostgreSQL
// implementation and its concrete conformance assertion, and a later
// service-adoption slice wires a caller to it. The legacy
// CatalogAdminRepository port (catalog_record.go) remains unchanged.
//
// Every method covers all 11 CatalogKindCode kinds generically through
// CatalogRecord.Kind rather than one method per kind, and is expected to
// perform CAS/insert/delete, reload and validate the complete 11-kind
// catalog in the same transaction, and return that reloaded, committed
// CatalogWriteResult — never an in-memory, not-yet-committed candidate
// (design "Catalog transaction and coherent publication", steps 1-7).
type CatalogAdminRepositoryV2 interface {
	// Insert creates rec at revision 1; creation has no expected revision.
	Insert(ctx context.Context, rec CatalogRecord) (CatalogWriteResult, error)

	// Update applies rec under optimistic concurrency: expectedRevision
	// must be non-zero and match the persisted revision, or the
	// implementation classifies the call as a stale-revision conflict
	// (design "SQL CAS classification").
	Update(ctx context.Context, rec CatalogRecord, expectedRevision uint64) (CatalogWriteResult, error)

	// SetActive performs a lifecycle transition — deactivate when active is
	// false, reactivate when true — under the same CAS discipline as
	// Update. An already-current expectedRevision with the requested state
	// already set is an idempotent no-op that publishes nothing new.
	SetActive(ctx context.Context, kind CatalogKindCode, id int64, active bool, expectedRevision uint64) (CatalogWriteResult, error)

	// Delete performs a conservative hard delete under the same CAS
	// discipline as Update; the returned CatalogWriteResult.Record is
	// always nil.
	Delete(ctx context.Context, kind CatalogKindCode, id int64, expectedRevision uint64) (CatalogWriteResult, error)
}
