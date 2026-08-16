package domain

import "context"

// CatalogRef identifies another catalog record by its own natural code (or
// "|"-joined composite identity, e.g. a Familia's "class|code") rather than
// a database ID. ApplyCatalogMutation (catalog_mutation.go) operates on the
// pure Go ResourceCatalog snapshot, which carries no ID concept at all
// (design D10) — every cross-reference inside that snapshot already resolves
// by the same natural keys (ClassCode, FamilyCode, ...), so CatalogRef
// follows the same convention rather than inventing a second identity
// system the pure domain layer cannot resolve without a repository.
type CatalogRef struct {
	Kind CatalogKindCode
	Code string
	// Label is the human-readable identity resolved by the repository for UI
	// presentation. Code remains the stable mutation/reference key.
	Label string
}

// CatalogValue is one field's value inside a CatalogRecord (design D8) — a
// typed union keyed by the owning FieldDescriptor.Kind, avoiding both the
// type-safety loss of map[string]any and the ~10x code blowup of one Go
// struct per kind.
type CatalogValue struct {
	Text string
	Bool bool
	Int  int
	List []string
	Ref  CatalogRef
}

// CatalogRecord is the generic transport shape for one row of any
// administrable CatalogKind (design D8). ID is the repository-assigned
// database identity (0 until Insert persists it) — the pure domain layer
// never uses it for cross-references, only the repository does.
type CatalogRecord struct {
	Kind   CatalogKindCode
	ID     int64
	Active bool
	Values map[string]CatalogValue
}

// CatalogFilter narrows a CatalogAdminRepository.List call: Text is a
// case-insensitive partial match against the kind's Searchable fields,
// Parent narrows by one or more of the kind's own scoping fields (e.g.
// {"class": ...} when listing Familias), and IncludeInactive opts into rows
// a filtered read query (FamiliesFor/TypesFor/OptionsFor/NaturalUnitsFor)
// would otherwise hide — the admin engine's own escape hatch for editing
// inactive rows (design Risk#3).
type CatalogFilter struct {
	Text            string
	Parent          map[string]CatalogValue
	IncludeInactive bool
	Limit           int
	Offset          int
}

// CatalogDependency reports how many rows of Kind reference one record,
// and whether that reference Blocks a guarded Desactivar/Delete (design §6).
type CatalogDependency struct {
	Kind     CatalogKindCode
	Count    int
	Blocking bool
}

// CatalogAdminRepository is the port every catalog-structure write goes
// through (design §3) — implemented in internal/postgres by a later PR
// (explicitly out of this PR's scope), consumed by internal/app/catalogo.
type CatalogAdminRepository interface {
	List(ctx context.Context, kind CatalogKindCode, f CatalogFilter) ([]CatalogRecord, error)
	Get(ctx context.Context, kind CatalogKindCode, id int64) (CatalogRecord, error)
	Insert(ctx context.Context, rec CatalogRecord) (int64, error)
	Update(ctx context.Context, rec CatalogRecord) error
	SetActive(ctx context.Context, kind CatalogKindCode, id int64, active bool) error
	Delete(ctx context.Context, kind CatalogKindCode, id int64) error
	Dependents(ctx context.Context, kind CatalogKindCode, id int64) ([]CatalogDependency, error)
	ReferencedByResources(ctx context.Context, kind CatalogKindCode, id int64) (bool, error)
}
