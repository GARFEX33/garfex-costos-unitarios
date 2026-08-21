package domain

import (
	"errors"
	"fmt"
	"testing"
)

// TestErrIdentityConflictWrapsIntegrityButNotViceVersa is the RED/TRIANGULATE
// evidence that ErrIdentityConflict is a strictly narrower, precedence-first
// classification of the broader ErrResourceIntegrity sentinel: every
// ErrIdentityConflict occurrence also satisfies the legacy
// errors.Is(err, ErrResourceIntegrity) check (source compatibility with
// existing identity-mismatch tests), but a genuine bare
// persistence/cardinality corruption error never also matches
// ErrIdentityConflict — proving the two are never accidentally collapsed in
// the wrong direction (design: cardinality corruption keeps using the OLD
// ErrResourceIntegrity, never redirected).
func TestIdentityConflictWrapsIntegrityButNotViceVersa(t *testing.T) {
	if !errors.Is(ErrIdentityConflict, ErrResourceIntegrity) {
		t.Fatal("ErrIdentityConflict does not satisfy errors.Is(_, ErrResourceIntegrity); existing identity-mismatch callers would break")
	}
	corruption := fmt.Errorf("%w: duplicate attribute row", ErrResourceIntegrity)
	if errors.Is(corruption, ErrIdentityConflict) {
		t.Fatal("a genuine ErrResourceIntegrity corruption error incorrectly also matches ErrIdentityConflict")
	}
}

// TestErrRevisionConflictAliasesResourceRepositoryV2Sentinel proves 5A
// reuses/consolidates 3H's CAS sentinel rather than duplicating it.
func TestRevisionConflictAliasesResourceRepositoryV2Sentinel(t *testing.T) {
	if !errors.Is(ErrRevisionConflict, ErrResourceRevisionConflict) || !errors.Is(ErrResourceRevisionConflict, ErrRevisionConflict) {
		t.Fatal("ErrRevisionConflict must alias the existing 3H domain.ErrResourceRevisionConflict sentinel, not duplicate it")
	}
}

func TestReactivationImpossibleWrapPreservesReasonAndIsNilSafe(t *testing.T) {
	if WrapReactivationImpossible(nil) != nil {
		t.Fatal("WrapReactivationImpossible(nil) must return nil")
	}
	wrapped := WrapReactivationImpossible(ErrResourceReference)
	if !errors.Is(wrapped, ErrReactivationImpossible) || !errors.Is(wrapped, ErrResourceReference) {
		t.Fatalf("WrapReactivationImpossible(%v) = %v, want it to satisfy errors.Is for both ErrReactivationImpossible and the wrapped reason", ErrResourceReference, wrapped)
	}
	if errors.Is(wrapped, ErrIdentityConflict) {
		t.Fatal("ErrReactivationImpossible must never also satisfy ErrIdentityConflict — identity disagreement is excluded per design")
	}
}

func TestCatalogInvalidWrapPreservesReasonAndIsNilSafe(t *testing.T) {
	if WrapInvalidCatalog(nil) != nil {
		t.Fatal("WrapInvalidCatalog(nil) must return nil")
	}
	wrapped := WrapInvalidCatalog(ErrResourceValidation)
	if !errors.Is(wrapped, ErrInvalidCatalog) || !errors.Is(wrapped, ErrResourceValidation) {
		t.Fatalf("WrapInvalidCatalog(%v) = %v, want it to satisfy errors.Is for both ErrInvalidCatalog and the wrapped reason", ErrResourceValidation, wrapped)
	}
}

// TestOutcomeSentinelsAreMutuallyDistinct is the non-collapse table: none of
// the five dedicated outcomes may accidentally alias another.
func TestOutcomeSentinelsAreMutuallyDistinct(t *testing.T) {
	sentinels := map[string]error{
		"ErrIdentityConflict":       ErrIdentityConflict,
		"ErrInvalidLifecycle":       ErrInvalidLifecycle,
		"ErrReactivationImpossible": ErrReactivationImpossible,
		"ErrInvalidCatalog":         ErrInvalidCatalog,
		"ErrRevisionConflict":       ErrRevisionConflict,
	}
	for aName, a := range sentinels {
		for bName, b := range sentinels {
			if aName == bName {
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("%s unexpectedly satisfies errors.Is(_, %s); the five dedicated outcomes must stay distinct", aName, bName)
			}
		}
	}
}
