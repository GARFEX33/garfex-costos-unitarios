package suppliercore

// LifecycleScope narrows a read to active, inactive, or all records. The
// empty value behaves as ScopeActive.
type LifecycleScope string

const (
	ScopeActive   LifecycleScope = "ACTIVE"
	ScopeInactive LifecycleScope = "INACTIVE"
	ScopeAll      LifecycleScope = "ALL"
)

// SupplierQuery narrows a Supplier search.
type SupplierQuery struct {
	Text   string
	Scope  LifecycleScope
	Limit  int
	Offset int
}

// BranchQuery narrows a Branch list. SupplierID is required — Branch has no
// independent search outside its owning Supplier.
type BranchQuery struct {
	SupplierID int64
	Text       string
	Scope      LifecycleScope
	Limit      int
	Offset     int
}

// ContactQuery narrows a Contact list. SupplierID is required. BranchID, when
// set, narrows further to contacts belonging to that one branch.
type ContactQuery struct {
	SupplierID int64
	BranchID   *int64
	Text       string
	Scope      LifecycleScope
	Limit      int
	Offset     int
}

// SupplierPage is one page of a Supplier search.
type SupplierPage struct {
	Query       SupplierQuery
	Suppliers   []Supplier
	HasPrevious bool
	HasNext     bool
}

// BranchPage is one page of a Branch list.
type BranchPage struct {
	Query       BranchQuery
	Branches    []Branch
	HasPrevious bool
	HasNext     bool
}

// ContactPage is one page of a Contact list.
type ContactPage struct {
	Query       ContactQuery
	Contacts    []Contact
	HasPrevious bool
	HasNext     bool
}
