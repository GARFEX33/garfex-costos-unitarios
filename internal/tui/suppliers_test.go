package tui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
)

type supplierListTestService struct {
	suppliers []domain.Supplier
	err       error
	criteria  domain.SupplierSearch
}

func (s *supplierListTestService) SearchSuppliers(_ context.Context, criteria domain.SupplierSearch) ([]domain.Supplier, error) {
	s.criteria = criteria
	return s.suppliers, s.err
}

func supplierModelAfter(t *testing.T, model SupplierModel, msg tea.Msg) SupplierModel {
	t.Helper()
	next, _ := model.Update(msg)
	return next.(SupplierModel)
}

func supplierKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func TestSupplierPR2AInitialActiveEmptyQueryLoad(t *testing.T) {
	service := &supplierListTestService{suppliers: []domain.Supplier{{ID: 41}, {ID: 42}}}
	model := NewSupplierModel(service)
	frame := model.CurrentFrame().(SupplierManagerFrame)

	if frame.Query != "" || frame.Filter != SupplierFilterActive || frame.State != SupplierStateLoading {
		t.Fatalf("initial frame = %#v, want active empty-query loading", frame)
	}
	if got := frame.StateText(); got != "Cargando…" {
		t.Fatalf("initial state text = %q, want loading text", got)
	}

	model = supplierModelAfter(t, model, model.Init()())
	frame = model.CurrentFrame().(SupplierManagerFrame)
	if service.criteria.Text != "" || service.criteria.Active == nil || !*service.criteria.Active {
		t.Fatalf("search criteria = %#v, want empty text and active=true", service.criteria)
	}
	if len(frame.Rows) != 2 || frame.Rows[0].ID != 41 || frame.SelectedID != 41 {
		t.Fatalf("loaded frame = %#v, want stable first supplier selection", frame)
	}
	if got := frame.StateText(); got != "Resultados" {
		t.Fatalf("result state text = %q, want Spanish result state", got)
	}
}

func TestSupplierPR2AEmptyAndErrorStatesAreSpanish(t *testing.T) {
	for _, test := range []struct {
		name    string
		service supplierListTestService
		want    string
	}{
		{name: "empty", want: "Sin resultados"},
		{name: "error", service: supplierListTestService{err: errors.New("secret")}, want: "No pude cargar los proveedores. Probá de nuevo en un momento."},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&test.service)
			model = supplierModelAfter(t, model, model.Init()())
			frame := model.CurrentFrame().(SupplierManagerFrame)
			if got := frame.StateText(); got != test.want {
				t.Fatalf("state text = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSupplierPR2AQueryAndFilterReset(t *testing.T) {
	service := &supplierListTestService{suppliers: []domain.Supplier{{ID: 7}}}
	model := NewSupplierModel(service)
	before := model.CurrentFrame().(SupplierManagerFrame)
	before.Rows = []SupplierRow{{ID: 99}}
	before.SelectedID, before.Cursor, before.Offset = 99, 4, 20
	model.frame = before

	model, cmd := model.LoadSuppliers("acme", SupplierFilterInactive)
	frame := model.CurrentFrame().(SupplierManagerFrame)
	if frame.Query != "acme" || frame.Filter != SupplierFilterInactive || len(frame.Rows) != 0 || frame.SelectedID != 0 || frame.Cursor != 0 || frame.Offset != 0 {
		t.Fatalf("reset frame = %#v, want cleared selection and pagination", frame)
	}
	if frame.State != SupplierStateLoading || cmd == nil {
		t.Fatalf("reset state/cmd = %v/%v, want loading and command", frame.State, cmd != nil)
	}

	model = supplierModelAfter(t, model, cmd())
	if service.criteria.Text != "acme" || service.criteria.Active == nil || *service.criteria.Active {
		t.Fatalf("filtered criteria = %#v, want inactive acme search", service.criteria)
	}
	if model.CurrentFrame().(SupplierManagerFrame).SelectedID != 7 {
		t.Fatalf("filtered selection = %d, want supplier 7", model.CurrentFrame().(SupplierManagerFrame).SelectedID)
	}

	model, cmd = model.LoadSuppliers("", SupplierFilterAll)
	model = supplierModelAfter(t, model, cmd())
	if service.criteria.Text != "" || service.criteria.Active != nil {
		t.Fatalf("all-suppliers criteria = %#v, want empty text and no active predicate", service.criteria)
	}
}

func TestSupplierPR2ACursorMovementKeepsStableSupplierSelection(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	frame := model.CurrentFrame().(SupplierManagerFrame)
	frame.Rows = []SupplierRow{{ID: 10}, {ID: 20}, {ID: 30}}
	frame.SelectedID, frame.Cursor, frame.State = 10, 0, SupplierStateReady
	model.frame = frame

	model = supplierModelAfter(t, model, supplierKey(tea.KeyDown))
	frame = model.CurrentFrame().(SupplierManagerFrame)
	if frame.Cursor != 1 || frame.SelectedID != 20 {
		t.Fatalf("down selection = cursor %d, supplier %d; want 1, 20", frame.Cursor, frame.SelectedID)
	}
	model = supplierModelAfter(t, model, supplierKey('j'))
	frame = model.CurrentFrame().(SupplierManagerFrame)
	if frame.Cursor != 2 || frame.SelectedID != 30 {
		t.Fatalf("j selection = cursor %d, supplier %d; want 2, 30", frame.Cursor, frame.SelectedID)
	}
	model = supplierModelAfter(t, model, supplierKey(tea.KeyUp))
	frame = model.CurrentFrame().(SupplierManagerFrame)
	if frame.Cursor != 1 || frame.SelectedID != 20 {
		t.Fatalf("up selection = cursor %d, supplier %d; want 1, 20", frame.Cursor, frame.SelectedID)
	}
}

func TestSupplierPR2AStaleRouteReplyIsRejected(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	current := model.CurrentFrame().(SupplierManagerFrame)
	model = supplierModelAfter(t, model, SupplierListMsg{RouteID: current.RouteID + 1, RequestID: current.RequestID, Rows: []SupplierRow{{ID: 99}}})
	frame := model.CurrentFrame().(SupplierManagerFrame)
	if frame.State != SupplierStateLoading || len(frame.Rows) != 0 {
		t.Fatalf("stale route changed frame = %#v", frame)
	}
}

func TestSupplierPR2AStaleRequestReplyIsRejected(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	current := model.CurrentFrame().(SupplierManagerFrame)
	model, _ = model.LoadSuppliers("new", SupplierFilterAll)
	model = supplierModelAfter(t, model, SupplierListMsg{RouteID: current.RouteID, RequestID: current.RequestID, Rows: []SupplierRow{{ID: 99}}})
	frame := model.CurrentFrame().(SupplierManagerFrame)
	if frame.State != SupplierStateLoading || len(frame.Rows) != 0 || frame.Query != "new" {
		t.Fatalf("stale request changed frame = %#v", frame)
	}
}

func TestSupplierPR2ARootEscReturnsWithoutShellExit(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	next, cmd := model.Update(supplierKey(tea.KeyEscape))
	model = next.(SupplierModel)

	if !model.AtRoot {
		t.Fatal("root Esc did not set the return signal")
	}
	if cmd != nil {
		t.Fatal("root Esc returned a shell command; wiring belongs to PR5")
	}
	if _, ok := model.CurrentFrame().(SupplierManagerFrame); !ok {
		t.Fatal("root Esc changed the manager frame")
	}
}
