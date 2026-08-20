package domain

import (
	"errors"
	"fmt"
)

// Stage 5 dedicated Core outcomes (design "Internal Core outcomes and
// mapping"). Existing errors previously collapsed four required semantics —
// identity disagreement, forbidden lifecycle transitions, impossible
// reactivation, and an invalid catalog authority — into broader sentinels
// declared elsewhere in this package (resource_types.go,
// resource_repository_v2.go). These five dedicated outcomes are added and
// EMITTED at their authoritative domain/application/adapter sites; they
// never cross a public boundary (there is no public package yet — Stage 6).

// ErrIdentityConflict is the dedicated outcome for a durable resource
// identity disagreeing with the Core's canonical identity — specifically a
// reactivation candidate's recomputed identity-v1 differing from the
// persisted one (recursos.Service.reactivationCandidate). It is never
// downgraded to a plain duplicate. For source-level errors.Is compatibility
// with existing identity-mismatch checks it WRAPS the pre-existing
// ErrResourceIntegrity sentinel, but the relationship is one-way: a genuine
// persistence/cardinality corruption keeps returning bare
// ErrResourceIntegrity, which never also satisfies
// errors.Is(err, ErrIdentityConflict). Classification MUST check
// ErrIdentityConflict before the broader ErrResourceIntegrity sentinel.
var ErrIdentityConflict = fmt.Errorf("resource identity conflict: %w", ErrResourceIntegrity)

// ErrInvalidLifecycle identifies an operation forbidden by the current
// lifecycle state — especially a hard delete of an active catalog record,
// or any other non-idempotent unsupported transition — distinct from a
// request-level business-rule ErrResourceValidation rejection.
var ErrInvalidLifecycle = errors.New("operation forbidden by current lifecycle state")

// ErrReactivationImpossible is the application-level reactivation outcome
// wrapping the internal reason a retained inactive resource cannot validate
// under the current active catalog, or cannot restore a required
// reference. Canonical/stored identity disagreement is excluded and
// returns ErrIdentityConflict instead — the two never both match the same
// error. Callers may further errors.Is the wrapped underlying reason
// (typically ErrResourceValidation or ErrResourceReference).
var ErrReactivationImpossible = errors.New("retained record cannot be reactivated under the current catalog")

// ErrInvalidCatalog wraps failures proving the current, candidate,
// transaction-reloaded, or independently loaded ResourceCatalog authority
// itself is structurally or semantically invalid or lossy (e.g. a
// candidate's own Validate() failing after a mutation is applied) —
// distinct from a request-level ErrResourceValidation/ErrCatalogReference
// rejection of one bad mutation, which keeps its existing sentinel.
var ErrInvalidCatalog = errors.New("catalog authority is invalid")

// ErrRevisionConflict is the dedicated stale-CAS sentinel. It is a value
// alias for the pre-existing domain.ErrResourceRevisionConflict (3H,
// resource_repository_v2.go) rather than a duplicate: both names identify
// the exact same error value, so errors.Is matches either interchangeably
// and every existing CAS emission site is reused, not re-implemented.
var ErrRevisionConflict = ErrResourceRevisionConflict

// WrapReactivationImpossible classifies reason — typically a catalog
// validation/reference failure surfaced while rebuilding or persisting a
// reactivation candidate — as ErrReactivationImpossible while preserving
// errors.Is access to reason itself. A nil reason returns nil.
func WrapReactivationImpossible(reason error) error {
	if reason == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrReactivationImpossible, reason)
}

// WrapInvalidCatalog classifies reason — a candidate/current/reloaded
// ResourceCatalog's own Validate() failure — as ErrInvalidCatalog while
// preserving errors.Is access to reason itself. A nil reason returns nil.
func WrapInvalidCatalog(reason error) error {
	if reason == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrInvalidCatalog, reason)
}
