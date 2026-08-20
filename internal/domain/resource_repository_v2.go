package domain

import (
	"context"
	"errors"
)

// ErrResourceRevisionConflict classifies a ResourceRepositoryV2 lifecycle
// call whose expectedRevision no longer matches the persisted revision,
// distinct from ErrResourceNotFound (absent row) and ErrResourceIntegrity
// (identity-v1 mismatch) — never inferred from an error string, mirroring
// catalog_admin_repository_v2.go's casUpdateRevision disambiguation
// (design "SQL CAS classification").
var ErrResourceRevisionConflict = errors.New("resource revision conflict")

// ResourceRepositoryV2 is the additive, revision-aware Resource lifecycle
// write port (design "Resource revisions and identity"), mirroring
// CatalogAdminRepositoryV2's additive-interface pattern for resources.
// Unlike stage 3D-3G's dormant catalog V2 internals, this is real and
// callable: recursos.Service type-asserts s.repo for it like it already
// does for ResourcePageRepository, so one domain.ResourceRepository
// implementation may additionally satisfy this interface with no legacy
// method/signature change. Method names are deliberately distinct from
// Deactivate/Reactivate so one concrete type implements both interfaces.
type ResourceRepositoryV2 interface {
	// DeactivateRevision performs a CAS deactivate: expectedRevision must be
	// non-zero and match the persisted revision, or the call is classified
	// ErrResourceRevisionConflict. An already-inactive resource at the
	// current revision is an idempotent no-op: it commits without
	// incrementing revision.
	DeactivateRevision(ctx context.Context, id int64, expectedRevision uint64) (Resource, error)

	// ReactivateRevision applies the same CAS discipline to transition back
	// to active, additionally requiring expectedIdentityKey to still match
	// the persisted identity-v1 (ErrResourceIntegrity otherwise); identity-v1
	// is never regenerated. An already-active resource at the current
	// revision is an idempotent no-op.
	ReactivateRevision(ctx context.Context, id int64, expectedIdentityKey string, expectedRevision uint64) (Resource, error)

	// UpdateRevision CAS-replaces the parent row and the complete attribute
	// set in one transaction, rolling both back on either failure.
	// identity_key is never written here.
	UpdateRevision(ctx context.Context, resource Resource, expectedRevision uint64) (Resource, error)
}
