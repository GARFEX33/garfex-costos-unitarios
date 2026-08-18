package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
)

type SupplierListService interface {
	SearchSuppliers(context.Context, domain.SupplierSearch) ([]domain.Supplier, error)
}

type SupplierDetailService interface {
	GetSupplier(context.Context, int64) (domain.Supplier, error)
	ListBranches(context.Context, int64, domain.ListCriteria) ([]domain.Branch, error)
	ListContacts(context.Context, int64, domain.ContactListCriteria) ([]domain.Contact, error)
}

type SupplierListMsg struct {
	RouteID, RequestID uint64
	Rows               []SupplierRow
	Err                error
}

type SupplierDetailMsg struct {
	RouteID, RequestID uint64
	Detail             SupplierDetail
	Err                error
}

const supplierPageSize = 25

func supplierListCmd(service SupplierListService, frame SupplierManagerFrame) tea.Cmd {
	return func() tea.Msg {
		values, err := service.SearchSuppliers(context.Background(), domain.SupplierSearch{
			Text: frame.Query, Active: supplierFilterActive(frame.Filter), Limit: supplierPageSize, Offset: frame.Offset,
		})
		rows := make([]SupplierRow, len(values))
		for i := range values {
			rows[i] = SupplierRow{ID: values[i].ID}
		}
		return SupplierListMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Rows: rows, Err: err}
	}
}

func supplierDetailCmd(service SupplierDetailService, frame SupplierDetailFrame) tea.Cmd {
	return func() tea.Msg {
		supplier, err := service.GetSupplier(context.Background(), frame.SupplierID)
		if err != nil {
			return SupplierDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Err: err}
		}
		active := true
		branches, err := service.ListBranches(context.Background(), frame.SupplierID, domain.ListCriteria{Active: &active, Limit: supplierPageSize})
		if err != nil {
			return SupplierDetailMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Err: err}
		}
		contacts, err := service.ListContacts(context.Background(), frame.SupplierID, domain.ContactListCriteria{Active: &active, Limit: supplierPageSize})
		return SupplierDetailMsg{
			RouteID: frame.RouteID, RequestID: frame.RequestID,
			Detail: SupplierDetail{Supplier: supplier, Branches: branches, Contacts: contacts}, Err: err,
		}
	}
}
