package domain

import "errors"

// Sentinel errors returned by a CatalogAdminRepository implementation
// (internal/postgres/catalog_admin_repository.go, design §3/§6). Distinct
// from ErrDuplicateResource/ErrResourceReference (resource_types.go,
// resource_catalog_validate.go), which are about resource *instances*
// (recursos), not catalog *structure* — keeping them separate avoids a
// confusing cross-domain error identity for callers that errors.Is against
// one or the other.
var (
	// ErrCatalogDuplicate is returned when an Insert/Update violates a
	// catalog-structure uniqueness constraint (e.g. a duplicate código).
	ErrCatalogDuplicate = errors.New("catalog record already exists")
	// ErrCatalogReference is returned when an Insert/Update names a ref
	// field (class/family/type/characteristic/optionSet/unit) that does not
	// resolve to an existing row.
	ErrCatalogReference = errors.New("catalog record reference invalid")
	// ErrCatalogInUse is returned when Delete is blocked by a foreign-key
	// reference from another catalog-structure row or a resource instance —
	// the repository-level backstop behind the service/UI guarded-delete
	// flow (design §6, PR7's job), never the primary UX.
	ErrCatalogInUse = errors.New("catalog record is still referenced and cannot be deleted")
)
