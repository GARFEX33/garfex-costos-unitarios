package resourcecore

import (
	"errors"
	"testing"
)

func TestPublicErrors(t *testing.T) {
	want := []ErrorCode{InvalidArgument, NotFound, Duplicate, InvalidReference, Validation, Integrity, IdentityConflict, InvalidLifecycle, ReactivationImpossible, InvalidCatalog, InUse, ImmutableCode, Conflict, Unavailable, Internal}
	seen := make(map[ErrorCode]bool)
	for _, c := range want {
		if seen[c] || c == "" {
			t.Fatalf("invalid error code %q", c)
		}
		seen[c] = true
	}
	if len(seen) != 15 {
		t.Fatalf("expected 15 error codes, got %d", len(seen))
	}
	err := NewError(NotFound, "resource missing")
	if err.Code() != NotFound || err.Error() != "resource missing" || Code(err) != NotFound || !IsCode(err, NotFound) || IsCode(err, Validation) {
		t.Fatalf("error mismatch")
	}
	if errors.Unwrap(err) != nil || Code(nil) != "" || IsCode(nil, NotFound) {
		t.Fatalf("unwrap or nil code mismatch")
	}
}
