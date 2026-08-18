package tui

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

type SupplierManagerFrame struct {
	RouteID, RequestID       uint64
	Query                    string
	Filter                   SupplierFilter
	Rows                     []SupplierRow
	SelectedID               int64
	Cursor, Offset, Viewport int
	State                    SupplierState
	Error                    string
}

func (f SupplierManagerFrame) StateText() string { return f.State.text(f.Error) }

func supplierFilterActive(filter SupplierFilter) *bool {
	if filter == SupplierFilterAll {
		return nil
	}
	active := filter == SupplierFilterActive
	return &active
}
