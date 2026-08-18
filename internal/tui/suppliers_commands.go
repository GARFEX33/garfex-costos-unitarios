package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
)

type SupplierListService interface {
	SearchSuppliers(context.Context, domain.SupplierSearch) ([]domain.Supplier, error)
}

type SupplierListMsg struct {
	RouteID, RequestID uint64
	Rows               []SupplierRow
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
