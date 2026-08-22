package suppliercore

import "time"

// Supplier is an independent commercial party.
type Supplier struct {
	ID            int64
	TradeName     string
	LegalName     string
	TaxIdentifier string
	Website       string
	Notes         string
	Active        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Branch belongs to exactly one Supplier. It has no independent identity.
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

// Contact belongs to one Supplier and may optionally belong to one of that
// same Supplier's branches. It has no independent identity.
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

// BranchKey identifies one Branch for read.
type BranchKey struct {
	SupplierID int64
	BranchID   int64
}

// ContactKey identifies one Contact for read.
type ContactKey struct {
	SupplierID int64
	ContactID  int64
}
