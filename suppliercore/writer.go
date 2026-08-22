package suppliercore

import (
	"context"
	"strings"
)

// WriteCapabilities is the service-shaped write seam a Writer delegates to.
// Only graduated operations appear here; a later per-operation change adds
// one method, which is an accepted per-slice seam change, not a consumer
// break.
type WriteCapabilities interface {
	CreateSupplier(context.Context, SupplierWriteRequest) (Supplier, error)
}

// Writer is the public write-only Supplier Master contract. It validates
// request shape and delegates writes to WriteCapabilities.
type Writer struct {
	cap WriteCapabilities
}

// NewWriter returns a Writer backed by cap. A nil capability is rejected
// with INVALID_ARGUMENT.
func NewWriter(cap WriteCapabilities) (*Writer, error) {
	if cap == nil {
		return nil, NewError(InvalidArgument, "write capabilities are required")
	}
	return &Writer{cap: cap}, nil
}

// CreateSupplier creates one supplier. Shape validation checks only that
// Actor is non-blank; whether the request carries enough content (trade
// name, legal name, or tax identifier) is domain.NewSupplier's sole
// authority and surfaces as VALIDATION, not a boundary rejection here.
func (w *Writer) CreateSupplier(ctx context.Context, req SupplierWriteRequest) (Supplier, error) {
	if err := validateSupplierWriteRequest(req); err != nil {
		return Supplier{}, err
	}
	s, err := w.cap.CreateSupplier(ctx, req)
	if err != nil {
		return Supplier{}, err
	}
	return CloneSupplier(s), nil
}

func validateSupplierWriteRequest(req SupplierWriteRequest) error {
	if strings.TrimSpace(req.Actor) == "" {
		return NewError(InvalidArgument, "actor is required")
	}
	return nil
}
