package domain

import (
	"errors"
	"fmt"
)

var (
	ErrValidation            = errors.New("supplier master validation failed")
	ErrNotFound              = errors.New("supplier master record not found")
	ErrConflict              = errors.New("supplier master conflict")
	ErrSupplierNotFound      = fmt.Errorf("%w: supplier", ErrNotFound)
	ErrBranchNotFound        = fmt.Errorf("%w: branch", ErrNotFound)
	ErrContactNotFound       = fmt.Errorf("%w: contact", ErrNotFound)
	ErrTaxIdentifierConflict = fmt.Errorf("%w: tax identifier", ErrConflict)
	ErrBranchOwnership       = fmt.Errorf("%w: branch does not belong to supplier", ErrValidation)
)

type ValidationError struct {
	Field   string
	Message string
}

func NewValidationError(field, message string) ValidationError {
	return ValidationError{Field: field, Message: message}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (ValidationError) Unwrap() error { return ErrValidation }
