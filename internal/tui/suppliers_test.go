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
	supplier  domain.Supplier
	branches  []domain.Branch
	contacts  []domain.Contact
	err       error
	criteria  domain.SupplierSearch
}

func (s *supplierListTestService) SearchSuppliers(_ context.Context, criteria domain.SupplierSearch) ([]domain.Supplier, error) {
	s.criteria = criteria
	return s.suppliers, s.err
}

func (s *supplierListTestService) GetSupplier(context.Context, int64) (domain.Supplier, error) {
	return s.supplier, s.err
}
func (s *supplierListTestService) ListBranches(context.Context, int64, domain.ListCriteria) ([]domain.Branch, error) {
	return s.branches, s.err
}
func (s *supplierListTestService) ListContacts(context.Context, int64, domain.ContactListCriteria) ([]domain.Contact, error) {
	return s.contacts, s.err
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

func TestSupplierPR2BSnapshotRestoresExactlyOneManagerLevel(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	manager := model.CurrentFrame().(SupplierManagerFrame)
	manager.Query = "acme"
	manager.Filter = SupplierFilterInactive
	manager.Rows = []SupplierRow{{ID: 7}, {ID: 8}}
	manager.SelectedID, manager.Cursor, manager.Offset, manager.Viewport = 8, 1, 25, 17
	model.frame = manager

	next, cmd := model.Update(supplierKey(tea.KeyEnter))
	model = next.(SupplierModel)
	if cmd == nil {
		t.Fatal("manager Enter returned no detail command")
	}
	if detail := model.CurrentFrame().(SupplierDetailFrame); detail.SupplierID != 8 || detail.State != SupplierDetailStateLoading {
		t.Fatalf("detail frame = %#v, want supplier 8 loading", detail)
	}

	next, _ = model.Update(supplierKey(tea.KeyEscape))
	model = next.(SupplierModel)
	restored := model.CurrentFrame().(SupplierManagerFrame)
	if restored.Query != manager.Query || restored.Filter != manager.Filter || restored.SelectedID != manager.SelectedID || restored.Cursor != manager.Cursor || restored.Offset != manager.Offset || restored.Viewport != manager.Viewport {
		t.Fatalf("restored manager = %#v, want snapshot %#v", restored, manager)
	}
}

func TestSupplierPR2BDetailCommandGroupsPresentationOnlyAndKeepsStableIDs(t *testing.T) {
	branchID := int64(20)
	service := &supplierListTestService{
		supplier: domain.Supplier{ID: 8, TradeName: "Acme", Active: true},
		branches: []domain.Branch{{ID: branchID, SupplierID: 8, Name: "Centro", City: "Rosario"}},
		contacts: []domain.Contact{
			{ID: 30, SupplierID: 8, Name: "General", BranchID: nil},
			{ID: 31, SupplierID: 8, Name: "Branch contact", BranchID: &branchID},
		},
	}
	model := NewSupplierModel(service)
	manager := model.CurrentFrame().(SupplierManagerFrame)
	manager.Rows, manager.SelectedID = []SupplierRow{{ID: 8}}, 8
	model.frame = manager
	next, cmd := model.Update(supplierKey(tea.KeyEnter))
	model = next.(SupplierModel)
	model = supplierModelAfter(t, model, cmd())

	detail := model.CurrentFrame().(SupplierDetailFrame)
	if detail.State != SupplierDetailStateReady || len(detail.Items) != 5 {
		t.Fatalf("detail = %#v, want ready grouped detail", detail)
	}
	if got := detail.Items[0]; got.Kind != SupplierDetailHeading || got.Selectable || got.Label != "General" {
		t.Fatalf("general heading = %#v", got)
	}
	if got := detail.Items[2]; got.Kind != SupplierDetailHeading || got.Selectable || got.Label != "Rosario" {
		t.Fatalf("city heading = %#v", got)
	}
	if got := detail.Items[1]; got.Target.ContactID != 30 || got.Kind != SupplierDetailContact {
		t.Fatalf("general contact = %#v", got)
	}
	if got := detail.Items[3]; got.Target.BranchID != branchID || got.Kind != SupplierDetailBranch || !got.Selectable {
		t.Fatalf("branch target = %#v", got)
	}
	if got := detail.Items[4]; got.Target.ContactID != 31 || got.Kind != SupplierDetailContact {
		t.Fatalf("branch contact = %#v", got)
	}
}

func TestSupplierPR2BEnterSelectsDirectChildButNotHeading(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	model.frame = SupplierDetailFrame{Items: []SupplierDetailItem{{Kind: SupplierDetailHeading, Label: "General"}, {Kind: SupplierDetailBranch, Selectable: true, Target: SupplierNavigationTarget{BranchID: 20}}}, State: SupplierDetailStateReady}
	model = supplierModelAfter(t, model, supplierKey(tea.KeyEnter))
	if _, ok := model.PendingNavigation(); ok {
		t.Fatal("heading Enter produced a navigation target")
	}
	model = supplierModelAfter(t, model, supplierKey(tea.KeyDown))
	model = supplierModelAfter(t, model, supplierKey(tea.KeyEnter))
	if got, ok := model.PendingNavigation(); !ok || got != (SupplierNavigationTarget{BranchID: 20}) {
		t.Fatalf("child navigation = %#v/%v, want branch target", got, ok)
	}
}

func TestSupplierPR2BStaleDetailReplyIsRejected(t *testing.T) {
	for _, test := range []struct {
		name string
		msg  SupplierDetailMsg
	}{
		{name: "route", msg: SupplierDetailMsg{RouteID: 3, RequestID: 9, Detail: SupplierDetail{Supplier: domain.Supplier{ID: 8}, Branches: []domain.Branch{{ID: 99}}}}},
		{name: "request", msg: SupplierDetailMsg{RouteID: 4, RequestID: 8, Detail: SupplierDetail{Supplier: domain.Supplier{ID: 8}, Branches: []domain.Branch{{ID: 99}}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = SupplierDetailFrame{RouteID: 4, RequestID: 9, SupplierID: 8, State: SupplierDetailStateLoading}
			next, _ := model.Update(test.msg)
			detail := next.(SupplierModel).CurrentFrame().(SupplierDetailFrame)
			if detail.State != SupplierDetailStateLoading || len(detail.Items) != 0 {
				t.Fatalf("stale detail reply changed frame = %#v", detail)
			}
		})
	}
}

func TestSupplierPR2BDetailStatesAreSpanish(t *testing.T) {
	for _, test := range []struct {
		name string
		msg  SupplierDetailMsg
		want string
	}{
		{name: "empty", msg: SupplierDetailMsg{RouteID: 1, RequestID: 1, Detail: SupplierDetail{}}, want: "Sin información"},
		{name: "error", msg: SupplierDetailMsg{RouteID: 1, RequestID: 1, Err: errors.New("secret")}, want: "No pude cargar el detalle del proveedor. Probá de nuevo en un momento."},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = SupplierDetailFrame{RouteID: 1, RequestID: 1, SupplierID: 8, State: SupplierDetailStateLoading}
			next, _ := model.Update(test.msg)
			detail := next.(SupplierModel).CurrentFrame().(SupplierDetailFrame)
			if got := detail.StateText(); got != test.want {
				t.Fatalf("detail state text = %q, want %q", got, test.want)
			}
		})
	}
}
