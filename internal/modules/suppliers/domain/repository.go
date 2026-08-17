package domain

import "context"

type SupplierSearch struct {
	Text   string
	Active *bool
	Limit  int
	Offset int
}

type ListCriteria struct {
	Active *bool
	Limit  int
	Offset int
}

type ContactListCriteria struct {
	Active   *bool
	BranchID *int64
	Limit    int
	Offset   int
}

// Repository is the persistence port for the complete Supplier Master core.
// Every child read and write is explicitly scoped by SupplierID.
type Repository interface {
	CreateSupplier(context.Context, Supplier) (Supplier, error)
	GetSupplier(context.Context, int64) (Supplier, error)
	SearchSuppliers(context.Context, SupplierSearch) ([]Supplier, error)
	UpdateSupplier(context.Context, Supplier) (Supplier, error)
	SetSupplierActive(context.Context, int64, bool) (Supplier, error)

	CreateBranch(context.Context, Branch) (Branch, error)
	GetBranch(context.Context, int64, int64) (Branch, error)
	ListBranches(context.Context, int64, ListCriteria) ([]Branch, error)
	UpdateBranch(context.Context, Branch) (Branch, error)
	SetBranchActive(context.Context, int64, int64, bool) (Branch, error)

	CreateContact(context.Context, Contact) (Contact, error)
	GetContact(context.Context, int64, int64) (Contact, error)
	ListContacts(context.Context, int64, ContactListCriteria) ([]Contact, error)
	UpdateContact(context.Context, Contact) (Contact, error)
	SetContactActive(context.Context, int64, int64, bool) (Contact, error)
}
