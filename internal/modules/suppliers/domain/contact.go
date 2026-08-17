package domain

import (
	"strings"
	"time"
)

// Contact belongs to one Supplier and may optionally belong to one of that
// same Supplier's branches.
type Contact struct {
	ID         int64
	SupplierID int64
	BranchID   *int64
	Name       string
	Role       string
	Phone      string
	Mobile     string
	Email      string
	Notes      string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ContactDetails struct {
	BranchID *int64
	Name     string
	Role     string
	Phone    string
	Mobile   string
	Email    string
	Notes    string
}

func NewContact(supplierID int64, details ContactDetails) (Contact, error) {
	if supplierID <= 0 {
		return Contact{}, NewValidationError("supplier_id", "must be positive")
	}
	details = details.canonical()
	if details.BranchID != nil && *details.BranchID <= 0 {
		return Contact{}, NewValidationError("branch_id", "must be positive when provided")
	}
	if details.Name == "" {
		return Contact{}, NewValidationError("name", "is required")
	}
	return Contact{SupplierID: supplierID, BranchID: copyID(details.BranchID), Name: details.Name, Role: details.Role, Phone: details.Phone, Mobile: details.Mobile, Email: details.Email, Notes: details.Notes, Active: true}, nil
}

func (c Contact) WithDetails(details ContactDetails) (Contact, error) {
	next, err := NewContact(c.SupplierID, details)
	if err != nil {
		return Contact{}, err
	}
	c.BranchID, c.Name, c.Role = next.BranchID, next.Name, next.Role
	c.Phone, c.Mobile, c.Email, c.Notes = next.Phone, next.Mobile, next.Email, next.Notes
	return c, nil
}

func (d ContactDetails) canonical() ContactDetails {
	d.BranchID = copyID(d.BranchID)
	d.Name = strings.TrimSpace(d.Name)
	d.Role = strings.TrimSpace(d.Role)
	d.Phone = strings.TrimSpace(d.Phone)
	d.Mobile = strings.TrimSpace(d.Mobile)
	d.Email = strings.TrimSpace(d.Email)
	d.Notes = strings.TrimSpace(d.Notes)
	return d
}

func copyID(id *int64) *int64 {
	if id == nil {
		return nil
	}
	value := *id
	return &value
}
