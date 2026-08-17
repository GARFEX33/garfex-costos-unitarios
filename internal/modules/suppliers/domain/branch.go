package domain

import (
	"strings"
	"time"
)

// Branch belongs to exactly one Supplier. City is descriptive data, never identity.
type Branch struct {
	ID           int64
	SupplierID   int64
	Name         string
	Reference    string
	City         string
	State        string
	Country      string
	Address      string
	GeneralPhone string
	GeneralEmail string
	Notes        string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type BranchDetails struct {
	Name         string
	Reference    string
	City         string
	State        string
	Country      string
	Address      string
	GeneralPhone string
	GeneralEmail string
	Notes        string
}

func NewBranch(supplierID int64, details BranchDetails) (Branch, error) {
	if supplierID <= 0 {
		return Branch{}, NewValidationError("supplier_id", "must be positive")
	}
	details = details.canonical()
	return Branch{SupplierID: supplierID, Name: details.Name, Reference: details.Reference, City: details.City, State: details.State, Country: details.Country, Address: details.Address, GeneralPhone: details.GeneralPhone, GeneralEmail: details.GeneralEmail, Notes: details.Notes, Active: true}, nil
}

func (b Branch) WithDetails(details BranchDetails) (Branch, error) {
	next, err := NewBranch(b.SupplierID, details)
	if err != nil {
		return Branch{}, err
	}
	b.Name, b.Reference, b.City, b.State, b.Country = next.Name, next.Reference, next.City, next.State, next.Country
	b.Address, b.GeneralPhone, b.GeneralEmail, b.Notes = next.Address, next.GeneralPhone, next.GeneralEmail, next.Notes
	return b, nil
}

func (d BranchDetails) canonical() BranchDetails {
	d.Name = strings.TrimSpace(d.Name)
	d.Reference = strings.TrimSpace(d.Reference)
	d.City = strings.TrimSpace(d.City)
	d.State = strings.TrimSpace(d.State)
	d.Country = strings.TrimSpace(d.Country)
	d.Address = strings.TrimSpace(d.Address)
	d.GeneralPhone = strings.TrimSpace(d.GeneralPhone)
	d.GeneralEmail = strings.TrimSpace(d.GeneralEmail)
	d.Notes = strings.TrimSpace(d.Notes)
	return d
}
