package domain

import (
	"errors"
	"testing"
)

func TestNewContactRequiresNameAndValidScope(t *testing.T) {
	invalidBranch := int64(-1)
	tests := []struct {
		name     string
		supplier int64
		details  ContactDetails
	}{
		{name: "missing supplier", details: ContactDetails{Name: "Ana"}},
		{name: "missing name", supplier: 1},
		{name: "invalid branch", supplier: 1, details: ContactDetails{Name: "Ana", BranchID: &invalidBranch}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewContact(tt.supplier, tt.details); !errors.Is(err, ErrValidation) {
				t.Fatalf("NewContact() error = %v, want ErrValidation", err)
			}
		})
	}
}
