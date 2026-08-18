package tui

import "github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"

type SupplierFilter uint8

const (
	SupplierFilterActive SupplierFilter = iota
	SupplierFilterInactive
	SupplierFilterAll
)

type SupplierState uint8

const (
	SupplierStateLoading SupplierState = iota
	SupplierStateReady
	SupplierStateEmpty
	SupplierStateError
)

func (s SupplierState) text(err string) string {
	switch s {
	case SupplierStateLoading:
		return "Cargando…"
	case SupplierStateReady:
		return "Resultados"
	case SupplierStateEmpty:
		return "Sin resultados"
	case SupplierStateError:
		return err
	default:
		return ""
	}
}

type SupplierRow struct{ ID int64 }

type SupplierDetailState uint8

const (
	SupplierDetailStateLoading SupplierDetailState = iota
	SupplierDetailStateReady
	SupplierDetailStateEmpty
	SupplierDetailStateError
)

func (s SupplierDetailState) text(err string) string {
	if s == SupplierDetailStateError {
		return err
	}
	return [...]string{"Cargando…", "Resultados", "Sin información"}[s]
}

type SupplierDetailItemKind uint8

const (
	SupplierDetailHeading SupplierDetailItemKind = iota
	SupplierDetailBranch
	SupplierDetailContact
)

type SupplierNavigationTarget struct{ SupplierID, BranchID, ContactID int64 }

type SupplierDetailItem struct {
	Kind       SupplierDetailItemKind
	Label      string
	Selectable bool
	Target     SupplierNavigationTarget
}

type SupplierDetail struct {
	Supplier domain.Supplier
	Branches []domain.Branch
	Contacts []domain.Contact
}

type SupplierManagerFrame struct {
	RouteID, RequestID       uint64
	Query                    string
	Filter                   SupplierFilter
	Rows                     []SupplierRow
	SelectedID               int64
	Cursor, Offset, Viewport int
	State                    SupplierState
	Error                    string
	SearchFocused            bool
}

func (f SupplierManagerFrame) StateText() string { return f.State.text(f.Error) }

type SupplierManagerSnapshot = SupplierManagerFrame

type SupplierDetailFrame struct {
	RouteID, RequestID uint64
	SupplierID         int64
	Supplier           domain.Supplier
	Items              []SupplierDetailItem
	Cursor             int
	State              SupplierDetailState
	Error              string
}

func (f SupplierDetailFrame) StateText() string { return f.State.text(f.Error) }

func supplierDetailItems(detail SupplierDetail) []SupplierDetailItem {
	items := make([]SupplierDetailItem, 0, len(detail.Branches)+len(detail.Contacts)+2)
	contactsByBranch := make(map[int64][]domain.Contact)
	var general []domain.Contact
	for _, contact := range detail.Contacts {
		if contact.BranchID == nil {
			general = append(general, contact)
			continue
		}
		contactsByBranch[*contact.BranchID] = append(contactsByBranch[*contact.BranchID], contact)
	}
	if len(general) > 0 {
		items = append(items, SupplierDetailItem{Kind: SupplierDetailHeading, Label: "General"})
		for _, contact := range general {
			items = append(items, supplierContactItem(contact))
		}
	}
	for _, branch := range detail.Branches {
		items = append(items, SupplierDetailItem{Kind: SupplierDetailHeading, Label: branch.City})
		items = append(items, SupplierDetailItem{
			Kind: SupplierDetailBranch, Label: branch.Name, Selectable: true,
			Target: SupplierNavigationTarget{SupplierID: branch.SupplierID, BranchID: branch.ID},
		})
		for _, contact := range contactsByBranch[branch.ID] {
			items = append(items, supplierContactItem(contact))
		}
	}
	return items
}

func supplierContactItem(contact domain.Contact) SupplierDetailItem {
	var branchID int64
	if contact.BranchID != nil {
		branchID = *contact.BranchID
	}
	return SupplierDetailItem{
		Kind: SupplierDetailContact, Label: contact.Name, Selectable: true,
		Target: SupplierNavigationTarget{SupplierID: contact.SupplierID, BranchID: branchID, ContactID: contact.ID},
	}
}

func supplierFilterActive(filter SupplierFilter) *bool {
	if filter == SupplierFilterAll {
		return nil
	}
	active := filter == SupplierFilterActive
	return &active
}

const SupplierEditCreate = false
const SupplierEditUpdate = true

const (
	SupplierFieldTradeName = iota
	SupplierFieldLegalName
	SupplierFieldTaxIdentifier
	SupplierFieldWebsite
	SupplierFieldNotes
)

var supplierFormFields = []string{
	"Nombre comercial",
	"Razón social",
	"Identificación fiscal/RFC",
	"Sitio web",
	"Notas",
}

// SupplierEditFrame is the separate progressive form for supplier-only data.
type SupplierEditFrame struct {
	RouteID, RequestID uint64
	SupplierID         int64
	Mode               bool
	Values             domain.SupplierDetails
	Focus              int
	Focused            bool
	Error              string
}

func newSupplierEditFrame(route, request uint64, supplier domain.Supplier, mode bool) SupplierEditFrame {
	return SupplierEditFrame{
		RouteID: route, RequestID: request, SupplierID: supplier.ID, Mode: mode,
		Values: domain.SupplierDetails{
			TradeName: supplier.TradeName, LegalName: supplier.LegalName,
			TaxIdentifier: supplier.TaxIdentifier, Website: supplier.Website, Notes: supplier.Notes,
		},
		Focused: true,
	}
}
