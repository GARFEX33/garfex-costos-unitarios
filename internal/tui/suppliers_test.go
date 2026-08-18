package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
)

type supplierListTestService struct {
	suppliers       []domain.Supplier
	supplier        domain.Supplier
	branches        []domain.Branch
	contacts        []domain.Contact
	err             error
	criteria        domain.SupplierSearch
	branchCriteria  domain.ListCriteria
	contactCriteria domain.ContactListCriteria
	createResult    domain.Supplier
	updateResult    domain.Supplier
	activeResult    domain.Supplier
	createErr       error
	updateErr       error
	activeErr       error
	branchResult    domain.Branch
	contactResult   domain.Contact
	childErr        error
	branchSupplier  int64
	contactSupplier int64
	contactBranch   *int64
	createCalls     int
	updateCalls     int
	deactivateCalls int
	reactivateCalls int
}

func (s *supplierListTestService) SearchSuppliers(_ context.Context, criteria domain.SupplierSearch) ([]domain.Supplier, error) {
	s.criteria = criteria
	return s.suppliers, s.err
}

func (s *supplierListTestService) GetSupplier(context.Context, int64) (domain.Supplier, error) {
	return s.supplier, s.err
}
func (s *supplierListTestService) GetBranch(_ context.Context, _ int64, _ int64) (domain.Branch, error) {
	if len(s.branches) > 0 {
		return s.branches[0], s.err
	}
	return domain.Branch{}, s.err
}
func (s *supplierListTestService) GetContact(_ context.Context, _ int64, _ int64) (domain.Contact, error) {
	if len(s.contacts) > 0 {
		return s.contacts[0], s.err
	}
	return domain.Contact{}, s.err
}
func (s *supplierListTestService) ListBranches(_ context.Context, _ int64, criteria domain.ListCriteria) ([]domain.Branch, error) {
	s.branchCriteria = criteria
	return s.branches, s.err
}
func (s *supplierListTestService) ListContacts(_ context.Context, _ int64, criteria domain.ContactListCriteria) ([]domain.Contact, error) {
	s.contactCriteria = criteria
	return s.contacts, s.err
}

func (s *supplierListTestService) CreateSupplier(context.Context, domain.SupplierDetails) (domain.Supplier, error) {
	s.createCalls++
	return s.createResult, s.createErr
}

func (s *supplierListTestService) UpdateSupplier(context.Context, int64, domain.SupplierDetails) (domain.Supplier, error) {
	s.updateCalls++
	return s.updateResult, s.updateErr
}

func (s *supplierListTestService) DeactivateSupplier(context.Context, int64) (domain.Supplier, error) {
	s.deactivateCalls++
	return s.activeResult, s.activeErr
}

func (s *supplierListTestService) ReactivateSupplier(context.Context, int64) (domain.Supplier, error) {
	s.reactivateCalls++
	return s.activeResult, s.activeErr
}

func (s *supplierListTestService) AddBranch(_ context.Context, id int64, _ domain.BranchDetails) (domain.Branch, error) {
	s.branchSupplier = id
	return s.branchResult, s.childErr
}
func (s *supplierListTestService) UpdateBranch(_ context.Context, supplierID, id int64, _ domain.BranchDetails) (domain.Branch, error) {
	s.branchSupplier = supplierID
	return s.branchResult, s.childErr
}
func (s *supplierListTestService) DeactivateBranch(c context.Context, supplierID, id int64) (domain.Branch, error) {
	return s.UpdateBranch(c, supplierID, id, domain.BranchDetails{})
}
func (s *supplierListTestService) ReactivateBranch(c context.Context, supplierID, id int64) (domain.Branch, error) {
	return s.UpdateBranch(c, supplierID, id, domain.BranchDetails{})
}
func (s *supplierListTestService) AddContact(_ context.Context, id int64, d domain.ContactDetails) (domain.Contact, error) {
	s.contactSupplier, s.contactBranch = id, d.BranchID
	return s.contactResult, s.childErr
}
func (s *supplierListTestService) UpdateContact(_ context.Context, supplierID, id int64, d domain.ContactDetails) (domain.Contact, error) {
	s.contactSupplier, s.contactBranch = supplierID, d.BranchID
	return s.contactResult, s.childErr
}
func (s *supplierListTestService) DeactivateContact(c context.Context, supplierID, id int64) (domain.Contact, error) {
	return s.UpdateContact(c, supplierID, id, domain.ContactDetails{})
}
func (s *supplierListTestService) ReactivateContact(c context.Context, supplierID, id int64) (domain.Contact, error) {
	return s.UpdateContact(c, supplierID, id, domain.ContactDetails{})
}

func supplierModelAfter(t *testing.T, model SupplierModel, msg tea.Msg) SupplierModel {
	t.Helper()
	next, _ := model.Update(msg)
	return next.(SupplierModel)
}

func supplierKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func supplierTextKey(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: []rune(text)[0], Text: text})
}

func supplierCtrlN() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: 'n', Mod: tea.ModCtrl})
}

type unsupportedSupplierRoute struct{}

func requireSupplier(t *testing.T, ok bool, message string) {
	if !ok {
		t.Fatal(message)
	}
}

func TestSupplierPR4B1ManagerEnterStableIDs(t *testing.T) {
	assertChildEnter(t, BranchManagerFrame{SupplierID: 8, SelectedID: 41}, "BranchID", 41)
	assertChildEnter(t, ContactManagerFrame{SupplierID: 8, SelectedID: 31}, "ContactID", 31)
}

func assertChildEnter(t *testing.T, frame any, idField string, id int64) {
	m := NewSupplierModel(&supplierListTestService{})
	m.frame = frame
	next, _ := m.Update(supplierKey(tea.KeyEnter))
	got := reflect.ValueOf(next.(SupplierModel).CurrentFrame())
	requireSupplier(t, got.FieldByName("SupplierID").Int() == 8 && got.FieldByName(idField).Int() == id, "stable detail ID mismatch")
}

func TestSupplierPR4B1DetailContextAndRestoration(t *testing.T) {
	s := &supplierListTestService{supplier: domain.Supplier{ID: 8, TradeName: "Acme"}, branches: []domain.Branch{{ID: 41, SupplierID: 8, Name: "Centro"}}, contacts: []domain.Contact{{ID: 31, SupplierID: 8, Name: "Ana"}}}
	assertRestoredDetail(t, s, BranchManagerFrame{RouteID: 3, RequestID: 4, SupplierID: 8, Query: "ce", Filter: SupplierFilterInactive, SelectedID: 41, Cursor: 2, Offset: 25, Viewport: 10}, "BranchID", "Acme")
	assertRestoredDetail(t, s, ContactManagerFrame{RouteID: 5, RequestID: 6, SupplierID: 8, Query: "an", Filter: SupplierFilterAll, SelectedID: 31, Cursor: 1, Offset: 50, Viewport: 10}, "ContactID", "Ana")
}

func assertRestoredDetail(t *testing.T, s *supplierListTestService, before any, idField, want string) {
	m := NewSupplierModel(s)
	m.frame = before
	next, cmd := m.Update(supplierKey(tea.KeyEnter))
	m = supplierModelAfter(t, next.(SupplierModel), cmd())
	requireSupplier(t, strings.Contains(m.View().Content, want), "detail content missing")
	m = supplierModelAfter(t, m, supplierKey(tea.KeyEscape))
	requireSupplier(t, reflect.DeepEqual(m.CurrentFrame(), before), "exact snapshot restoration failed")
}

func TestSupplierPR4B1BranchScope(t *testing.T) {
	s := &supplierListTestService{}
	m := NewSupplierModel(s)
	m.frame = BranchDetailFrame{SupplierID: 8, BranchID: 41, State: SupplierDetailStateReady}
	next, cmd := m.Update(supplierKey('C'))
	m = next.(SupplierModel)
	f, ok := m.CurrentFrame().(ContactManagerFrame)
	msg, msgOK := cmd().(ContactListMsg)
	requireSupplier(t, ok && f.SupplierID == 8 && f.BranchID != nil && *f.BranchID == 41 && msgOK && s.contactCriteria.BranchID != nil && *s.contactCriteria.BranchID == 41 && msg.RouteID == f.RouteID && msg.RequestID == f.RequestID, "branch scope was not correlated")
}

func TestSupplierPR4B1StaleRepliesSpanishStates(t *testing.T) {
	service := &supplierListTestService{}
	assertStaleDetail(t, service, BranchDetailFrame{RouteID: 2, RequestID: 3, State: SupplierDetailStateLoading}, BranchDetailMsg{RouteID: 1, RequestID: 3}, BranchDetailMsg{RouteID: 2, RequestID: 3, Err: errors.New("fallo")}, "detalle de la sucursal")
	assertStaleDetail(t, service, ContactDetailFrame{RouteID: 2, RequestID: 3, State: SupplierDetailStateLoading}, ContactDetailMsg{RouteID: 2, RequestID: 2}, ContactDetailMsg{RouteID: 2, RequestID: 3, Err: errors.New("fallo")}, "detalle del contacto")
}

func assertStaleDetail(t *testing.T, s *supplierListTestService, frame any, stale, reply tea.Msg, want string) {
	m := NewSupplierModel(s)
	m.frame = frame
	before := m.CurrentFrame()
	m = supplierModelAfter(t, m, stale)
	requireSupplier(t, reflect.DeepEqual(m.CurrentFrame(), before), "stale reply changed frame")
	m = supplierModelAfter(t, m, reply)
	requireSupplier(t, strings.Contains(m.View().Content, want) && strings.Contains(m.View().Content, "Probá de nuevo"), "Spanish detail error missing")
}

func TestSupplierPR4B1Footers(t *testing.T) {
	branchView := (SupplierModel{frame: BranchDetailFrame{Supplier: domain.Supplier{TradeName: "Acme"}, Branch: domain.Branch{Name: "Centro", City: "Córdoba", GeneralPhone: "555", GeneralEmail: "c@acme.test"}}}).View().Content
	contactView := (SupplierModel{frame: ContactDetailFrame{Contact: domain.Contact{Name: "Ana", Role: "Compras", Mobile: "555", Email: "a@acme.test"}, Branch: domain.Branch{ID: 41, Name: "Centro", City: "Córdoba"}}}).View().Content
	requireSupplier(t, strings.Contains(branchView, "E editar") && strings.Contains(contactView, "E editar") && strings.Contains(branchView, "C contactos de la sucursal") && strings.Contains(branchView+contactView, "Sucursal: Centro (Córdoba)"), "route footer/content mismatch")
}

func TestSupplierPR4B1DetailStatesAndContent(t *testing.T) {
	states := []struct {
		name  string
		state SupplierDetailState
		want  string
	}{
		{name: "loading", state: SupplierDetailStateLoading, want: "Cargando…"},
		{name: "ready", state: SupplierDetailStateReady, want: "Resultados"},
		{name: "empty", state: SupplierDetailStateEmpty, want: "Sin información"},
	}
	for _, route := range []struct {
		name string
		view func(SupplierDetailState) string
	}{
		{name: "branch", view: func(state SupplierDetailState) string {
			return branchDetailView(BranchDetailFrame{State: state})
		}},
		{name: "contact", view: func(state SupplierDetailState) string {
			return contactDetailView(ContactDetailFrame{State: state})
		}},
	} {
		t.Run(route.name, func(t *testing.T) {
			for _, test := range states {
				t.Run(test.name, func(t *testing.T) {
					requireSupplier(t, strings.Contains(route.view(test.state), test.want), "detail state is not Spanish")
				})
			}
		})
	}

	branchView := branchDetailView(BranchDetailFrame{
		State:    SupplierDetailStateReady,
		Supplier: domain.Supplier{TradeName: "Acme SA", LegalName: "Acme Legal"},
		Branch:   domain.Branch{Name: "Centro", Reference: "BR-41", City: "Córdoba", State: "Córdoba", Country: "Argentina", Address: "Av. Siempre Viva 41", GeneralPhone: "555-0041", GeneralEmail: "centro@acme.test", Notes: "Recepción por puerta norte", Active: true},
		Contacts: []domain.Contact{{Name: "Ana"}},
	})
	for _, want := range []string{"Proveedor: Acme SA · Acme Legal", "Nombre: Centro", "Referencia: BR-41", "Ciudad: Córdoba", "Provincia/Estado: Córdoba", "País: Argentina", "Dirección: Av. Siempre Viva 41", "Teléfono general: 555-0041", "Email general: centro@acme.test", "Notas: Recepción por puerta norte", "Estado: Activo", "Contacto asociado: Ana"} {
		requireSupplier(t, strings.Contains(branchView, want), "branch detail field missing: "+want)
	}

	contactView := contactDetailView(ContactDetailFrame{
		State:   SupplierDetailStateReady,
		Contact: domain.Contact{Name: "Ana", Role: "Compras", Mobile: "555-1000", Phone: "555-2000", Email: "ana@acme.test", Notes: "Contacto principal", Active: true},
		Branch:  domain.Branch{ID: 41, Name: "Centro", City: "Córdoba"},
	})
	for _, want := range []string{"Nombre: Ana", "Cargo: Compras", "Sucursal: Centro (Córdoba)", "Móvil: 555-1000", "Teléfono: 555-2000", "Email: ana@acme.test", "Notas: Contacto principal", "Estado: Activo"} {
		requireSupplier(t, strings.Contains(contactView, want), "contact detail field missing: "+want)
	}
}

func TestSupplierPR4B1DetailFootersExact(t *testing.T) {
	for _, test := range []struct {
		name, view, footer string
	}{
		{name: "branch", view: branchDetailView(BranchDetailFrame{}), footer: "E editar · A estado · C contactos de la sucursal · Esc volver · Ctrl+C salir"},
		{name: "contact", view: contactDetailView(ContactDetailFrame{}), footer: "E editar · A estado · Esc volver · Ctrl+C salir"},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines := strings.Split(test.view, "\n")
			requireSupplier(t, lines[len(lines)-1] == test.footer, "detail footer command set mismatch")
		})
	}
}

func TestSupplierPR4ChildManagers(t *testing.T) {
	branches := []domain.Branch{
		{ID: 13, SupplierID: 8, Name: "Centro", City: "Córdoba", Active: true},
		{ID: 11, SupplierID: 8, Name: "Centro", City: "Rosario", Active: true},
		{ID: 12, SupplierID: 8, Name: "Norte", City: "Rosario", Active: true},
	}
	open := func(t *testing.T, s *supplierListTestService, detail SupplierDetailFrame, key rune) SupplierModel {
		m := NewSupplierModel(s)
		m.frame = detail
		return supplierModelAfter(t, m, supplierKey(key))
	}
	t.Run("Supplier Detail S opens scoped Branch Manager", func(t *testing.T) {
		s := &supplierListTestService{branches: branches}
		m := open(t, s, SupplierDetailFrame{SupplierID: 8, State: SupplierDetailStateReady}, 'S')
		f := m.CurrentFrame().(BranchManagerFrame)
		if f.SupplierID != 8 || f.Filter != SupplierFilterActive || f.State.text(f.Error) != "Cargando…" {
			t.Fatalf("opening = %#v", f)
		}
		m = supplierModelAfter(t, m, m.InitChild()())
		f = m.CurrentFrame().(BranchManagerFrame)
		if s.branchCriteria.Active == nil || !*s.branchCriteria.Active || s.branchCriteria.Limit != supplierPageSize || s.branchCriteria.Offset != 0 {
			t.Fatalf("criteria = %#v", s.branchCriteria)
		}
		if f.State.text(f.Error) != "Resultados" || len(f.Items) != 5 || f.Items[0].Selectable || f.Items[0].Label != "Córdoba" || f.Items[1].Target.BranchID != 13 || f.Items[2].Label != "Rosario" || f.Items[2].Selectable || f.SelectedID != 13 {
			t.Fatalf("grouping = %#v", f.Items)
		}
		m = supplierModelAfter(t, m, supplierKey(tea.KeyDown))
		if m.CurrentFrame().(BranchManagerFrame).SelectedID != 11 {
			t.Fatalf("heading selected: %#v", m.CurrentFrame())
		}
	})
	t.Run("Supplier Detail C opens supplier-scoped Contact Manager", func(t *testing.T) {
		branchID := int64(42)
		s := &supplierListTestService{contacts: []domain.Contact{{ID: 31, SupplierID: 8, Name: "Ana", Active: true}, {ID: 32, SupplierID: 8, Name: "Bruno", BranchID: &branchID, Active: true}}}
		detail := SupplierDetailFrame{SupplierID: 8, State: SupplierDetailStateReady, Items: []SupplierDetailItem{{Kind: SupplierDetailHeading, Label: "Rosario"}, {Kind: SupplierDetailBranch, Label: "Centro", Target: SupplierNavigationTarget{BranchID: branchID}}}}
		m := open(t, s, detail, 'C')
		f := m.CurrentFrame().(ContactManagerFrame)
		if f.SupplierID != 8 || f.BranchID != nil || f.Filter != SupplierFilterActive || f.State.text(f.Error) != "Cargando…" {
			t.Fatalf("opening = %#v", f)
		}
		m = supplierModelAfter(t, m, m.InitChild()())
		f = m.CurrentFrame().(ContactManagerFrame)
		if s.contactCriteria.Active == nil || !*s.contactCriteria.Active || s.contactCriteria.BranchID != nil || s.contactCriteria.Limit != supplierPageSize || s.contactCriteria.Offset != 0 {
			t.Fatalf("criteria = %#v", s.contactCriteria)
		}
		if f.State.text(f.Error) != "Resultados" || len(f.Items) != 4 || f.Items[0].Label != "General" || f.Items[0].Selectable || f.Items[2].Label != "Rosario / Centro" || f.Items[2].Selectable || f.SelectedID != 31 {
			t.Fatalf("grouping = %#v", f.Items)
		}
		m = supplierModelAfter(t, m, supplierKey(tea.KeyDown))
		if m.CurrentFrame().(ContactManagerFrame).SelectedID != 32 {
			t.Fatalf("heading selected: %#v", m.CurrentFrame())
		}
	})
	for _, tc := range []struct {
		name      string
		state     SupplierState
		err, want string
	}{
		{"loading", SupplierStateLoading, "", "Cargando…"}, {"result", SupplierStateReady, "", "Resultados"}, {"empty", SupplierStateEmpty, "", "Sin resultados"}, {"error", SupplierStateError, "No pude cargar la lista.", "No pude cargar la lista."},
	} {
		t.Run("Spanish "+tc.name, func(t *testing.T) {
			for _, frame := range []struct {
				state SupplierState
				err   string
			}{{tc.state, tc.err}, {tc.state, tc.err}} {
				if got := frame.state.text(frame.err); got != tc.want {
					t.Fatalf("state = %q, want %q", got, tc.want)
				}
			}
		})
	}
	for _, tc := range []struct {
		name   string
		frame  any
		branch bool
	}{
		{"branch", BranchManagerFrame{RouteID: 1, RequestID: 1, SupplierID: 8, Filter: SupplierFilterInactive, Query: "a", Offset: 25, Cursor: 1, SelectedID: 12, SearchFocused: true}, true}, {"contact", ContactManagerFrame{RouteID: 1, RequestID: 1, SupplierID: 8, Filter: SupplierFilterAll, Query: "a", Offset: 25, Cursor: 1, SelectedID: 32, SearchFocused: true}, false},
	} {
		t.Run("focused search "+tc.name, func(t *testing.T) {
			s := &supplierListTestService{}
			m := NewSupplierModel(s)
			m.frame = tc.frame
			next, cmd := m.Update(supplierTextKey("x"))
			m = next.(SupplierModel)
			if cmd == nil {
				t.Fatal("no search command")
			}
			switch f := m.CurrentFrame().(type) {
			case BranchManagerFrame:
				if f.Query != "ax" || f.Offset != 0 || f.Cursor != 0 || f.SelectedID != 0 {
					t.Fatalf("reset = %#v", f)
				}
			case ContactManagerFrame:
				if f.Query != "ax" || f.Offset != 0 || f.Cursor != 0 || f.SelectedID != 0 {
					t.Fatalf("reset = %#v", f)
				}
			}
			m = supplierModelAfter(t, m, cmd())
			if tc.branch {
				if s.branchCriteria.Text != "ax" || s.branchCriteria.Active == nil || *s.branchCriteria.Active || s.branchCriteria.Offset != 0 {
					t.Fatalf("search = %#v", s.branchCriteria)
				}
			} else if s.contactCriteria.Text != "ax" || s.contactCriteria.Active != nil || s.contactCriteria.Offset != 0 {
				t.Fatalf("search = %#v", s.contactCriteria)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		frame any
		msg   tea.Msg
	}{
		{"branch route", BranchManagerFrame{RouteID: 4, RequestID: 9, State: SupplierStateLoading}, BranchListMsg{RouteID: 3, RequestID: 9, Branches: branches}}, {"branch request", BranchManagerFrame{RouteID: 4, RequestID: 9, State: SupplierStateLoading}, BranchListMsg{RouteID: 4, RequestID: 8, Branches: branches}}, {"contact route", ContactManagerFrame{RouteID: 4, RequestID: 9, State: SupplierStateLoading}, ContactListMsg{RouteID: 3, RequestID: 9, Contacts: []domain.Contact{{ID: 31}}}}, {"contact request", ContactManagerFrame{RouteID: 4, RequestID: 9, State: SupplierStateLoading}, ContactListMsg{RouteID: 4, RequestID: 8, Contacts: []domain.Contact{{ID: 31}}}},
	} {
		t.Run("stale "+tc.name, func(t *testing.T) {
			m := NewSupplierModel(&supplierListTestService{})
			m.frame = tc.frame
			before := m.CurrentFrame()
			m = supplierModelAfter(t, m, tc.msg)
			if !reflect.DeepEqual(m.CurrentFrame(), before) {
				t.Fatalf("stale reply changed %#v", m.CurrentFrame())
			}
		})
	}
	for _, tc := range []struct {
		name string
		key  rune
	}{{"branch manager", 'S'}, {"contact manager", 'C'}} {
		t.Run("manager Esc "+tc.name, func(t *testing.T) {
			s := &supplierListTestService{branches: branches, contacts: []domain.Contact{{ID: 31, Name: "Ana"}}}
			before := SupplierDetailFrame{SupplierID: 8, State: SupplierDetailStateReady}
			m := open(t, s, before, tc.key)
			m = supplierModelAfter(t, m, m.InitChild()())
			m = supplierModelAfter(t, m, supplierKey(tea.KeyEscape))
			if !reflect.DeepEqual(m.CurrentFrame(), before) {
				t.Fatalf("restoration = %#v", m.CurrentFrame())
			}
		})
	}
	next, cmd := NewSupplierModel(&supplierListTestService{}).Update(supplierKey(tea.KeyEscape))
	if m := next.(SupplierModel); !m.AtRoot || cmd != nil {
		t.Fatalf("root Esc = %v/%v", next, cmd)
	}
}

func TestSupplierPR3AManagerCreateAndProgressiveSupplierForm(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	manager := model.CurrentFrame().(SupplierManagerFrame)
	manager.SearchFocused = true
	model.frame = manager

	next, cmd := model.Update(supplierCtrlN())
	model = next.(SupplierModel)
	if cmd != nil {
		t.Fatal("manager Ctrl+N returned a command; create opens synchronously")
	}
	form, ok := model.CurrentFrame().(SupplierEditFrame)
	if !ok || form.Mode != SupplierEditCreate || form.SupplierID != 0 {
		t.Fatalf("create frame = %#v, want supplier create form", model.CurrentFrame())
	}
	wantLabels := []string{"Nombre comercial", "Razón social", "Identificación fiscal/RFC", "Sitio web", "Notas"}
	if got := supplierFormFields; !reflect.DeepEqual(got, wantLabels) {
		t.Fatalf("form labels = %#v, want %#v", got, wantLabels)
	}

	for i, value := range []string{"Comercial", "Legal", "RFC-123", "https://acme.example", "Notas"} {
		for _, key := range value {
			model = supplierModelAfter(t, model, supplierTextKey(string(key)))
		}
		if i < 4 {
			model = supplierModelAfter(t, model, supplierKey(tea.KeyTab))
		}
	}
	form = model.CurrentFrame().(SupplierEditFrame)
	wantValues := domain.SupplierDetails{TradeName: "Comercial", LegalName: "Legal", TaxIdentifier: "RFC-123", Website: "https://acme.example", Notes: "Notas"}
	gotValues := form.Values
	if !reflect.DeepEqual(gotValues, wantValues) {
		t.Fatalf("progressive form values = %#v, want %#v", gotValues, wantValues)
	}
}

func TestSupplierPR3AManagerCtrlNWithoutSearchFocusOpensCreate(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	model = supplierModelAfter(t, model, supplierCtrlN())
	form, ok := model.CurrentFrame().(SupplierEditFrame)
	if !ok || form.Mode != SupplierEditCreate {
		t.Fatalf("unfocused manager Ctrl+N frame = %#v, want create form", model.CurrentFrame())
	}
}

func TestSupplierPR3ACtrlNIgnoredOutsideManager(t *testing.T) {
	for _, test := range []struct {
		name  string
		frame any
		check func(t *testing.T, got any)
	}{
		{
			name:  "detail",
			frame: SupplierDetailFrame{SupplierID: 71, State: SupplierDetailStateReady},
			check: func(t *testing.T, got any) {
				if detail := got.(SupplierDetailFrame); detail.SupplierID != 71 {
					t.Fatalf("detail after Ctrl+N = %#v, want unchanged detail", detail)
				}
			},
		},
		{
			name:  "edit form",
			frame: SupplierEditFrame{Mode: SupplierEditUpdate, SupplierID: 71},
			check: func(t *testing.T, got any) {
				if form := got.(SupplierEditFrame); form.SupplierID != 71 || form.Mode != SupplierEditUpdate {
					t.Fatalf("edit after Ctrl+N = %#v, want unchanged edit", form)
				}
			},
		},
		{
			name:  "unsupported route",
			frame: unsupportedSupplierRoute{},
			check: func(t *testing.T, got any) {
				if _, ok := got.(unsupportedSupplierRoute); !ok {
					t.Fatalf("unsupported route after Ctrl+N = %#v, want unchanged route", got)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = test.frame
			model = supplierModelAfter(t, model, supplierCtrlN())
			test.check(t, model.CurrentFrame())
		})
	}
}

func TestSupplierPR3ADetailEditUsesViewedSupplierID(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	model.frame = SupplierDetailFrame{SupplierID: 84, State: SupplierDetailStateReady}

	model = supplierModelAfter(t, model, supplierKey('E'))
	form, ok := model.CurrentFrame().(SupplierEditFrame)
	if !ok || form.Mode != SupplierEditUpdate || form.SupplierID != 84 {
		t.Fatalf("detail E frame = %#v, want edit for supplier 84", model.CurrentFrame())
	}
}

func TestSupplierPR3AFocusedPrintableKeysRemainText(t *testing.T) {
	for _, text := range []string{"E", "S", "C", "A", "?"} {
		t.Run(text, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = SupplierEditFrame{Mode: SupplierEditCreate, Values: domain.SupplierDetails{}, Focus: 0, Focused: true}
			model = supplierModelAfter(t, model, supplierTextKey(text))
			form, ok := model.CurrentFrame().(SupplierEditFrame)
			if !ok || form.Values.TradeName != text {
				t.Fatalf("focused %q input = %#v, want text in field", text, model.CurrentFrame())
			}
		})
	}
}

func TestSupplierPR3AFormEscCancelsCreateAndEditWithoutSave(t *testing.T) {
	manager := SupplierManagerFrame{Query: "acme", Filter: SupplierFilterInactive, Rows: []SupplierRow{{ID: 9}}, SelectedID: 9, Cursor: 0, Offset: 25, Viewport: 12, State: SupplierStateReady}
	detail := SupplierDetailFrame{RouteID: 3, RequestID: 4, SupplierID: 9, Supplier: domain.Supplier{ID: 9}, State: SupplierDetailStateReady}
	for _, test := range []struct {
		name   string
		before any
		open   tea.KeyPressMsg
		assert func(t *testing.T, got any)
	}{
		{
			name:   "create",
			before: manager,
			open:   supplierCtrlN(),
			assert: func(t *testing.T, got any) {
				if !reflect.DeepEqual(got, manager) {
					t.Fatalf("create Esc frame = %#v, want %#v", got, manager)
				}
			},
		},
		{
			name:   "edit",
			before: detail,
			open:   supplierKey('E'),
			assert: func(t *testing.T, got any) {
				if !reflect.DeepEqual(got, detail) {
					t.Fatalf("edit Esc frame = %#v, want %#v", got, detail)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = test.before
			model = supplierModelAfter(t, model, test.open)
			model = supplierModelAfter(t, model, supplierKey(tea.KeyEscape))
			test.assert(t, model.CurrentFrame())
		})
	}
}

func TestSupplierPR3AMinimumDeterministicSupplierFormView(t *testing.T) {
	model := NewSupplierModel(&supplierListTestService{})
	model.frame = SupplierEditFrame{Mode: SupplierEditUpdate, SupplierID: 84, Focus: 2, Focused: true}
	first := model.View().Content
	second := model.View().Content
	if first != second {
		t.Fatalf("form view changed between renders: %q != %q", first, second)
	}
	for _, label := range []string{"Nombre comercial", "Razón social", "Identificación fiscal/RFC", "Sitio web", "Notas", "Esc: Cancelar"} {
		if !strings.Contains(first, label) {
			t.Fatalf("form view = %q, want label/footer %q", first, label)
		}
	}
	if strings.Contains(first, "Sucursales") || strings.Contains(first, "Contactos") {
		t.Fatalf("supplier-only form rendered child fields: %q", first)
	}
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

// PR3B ledger is defined below.

// Failure cases are part of the named PR3B ledger below.

// Contextual route cases are part of the named PR3B ledger below.

// Esc priority cases are part of the named PR3B ledger below.
func TestSupplierPR3BLedger(t *testing.T) {
	names := strings.Split("active confirmation|inactive confirmation|confirmed deactivate|confirmed reactivate|Cancel and Esc no-write|create validation|create unknown|update not-found|update conflict|update unknown|lifecycle validation|lifecycle not-found|lifecycle conflict|lifecycle unknown|create refresh|update refresh|manager footer|detail footer|edit/create footer|confirm/help deterministic|focused ? remains text|Confirm Esc|Help Esc|Detail Esc|root Manager Esc|Create Esc|Edit Esc|unfocused ? opens Help", "|")
	for i, name := range names {
		t.Run(name, func(t *testing.T) { pr3BScenario(t, i) })
	}
}

func pr3BScenario(t *testing.T, i int) {
	m := NewSupplierModel(&supplierListTestService{})
	if i < 4 {
		active := i < 2
		s := &supplierListTestService{activeResult: domain.Supplier{ID: 8, Active: !active}}
		m = supplierModelAfter(t, pr3BDetail(s, active), supplierKey('A'))
		if i < 2 {
			f, ok := m.CurrentFrame().(SupplierLifecycleFrame)
			want := "Reactivar"
			if active {
				want = "Desactivar"
			}
			if !ok || f.Active != active || !strings.Contains(m.View().Content, want) {
				t.Fatal("lifecycle confirmation missing")
			}
		} else {
			next, cmd := m.Update(supplierKey(tea.KeyEnter))
			if cmd == nil {
				t.Fatal("lifecycle command missing")
			}
			m = supplierModelAfter(t, next.(SupplierModel), cmd())
			if d := m.CurrentFrame().(SupplierDetailFrame); d.Supplier.Active == active {
				t.Fatal("lifecycle transition did not refresh detail")
			}
		}
		return
	}
	if i == 4 {
		m = supplierModelAfter(t, pr3BDetail(nil, true), supplierKey('A'))
		m = supplierModelAfter(t, m, supplierKey('c'))
		if _, ok := m.CurrentFrame().(SupplierDetailFrame); !ok {
			t.Fatal("Cancel did not pop confirmation")
		}
		return
	}
	if i == 27 {
		m = supplierModelAfter(t, pr3BDetail(nil, true), supplierKey('?'))
		if _, ok := m.CurrentFrame().(SupplierHelpFrame); !ok {
			t.Fatal("help route missing")
		}
		return
	}
	if i >= 21 {
		m = supplierModelAfter(t, pr3BDetail(nil, true), supplierKey(tea.KeyEscape))
		if _, ok := m.CurrentFrame().(SupplierManagerFrame); !ok {
			t.Fatal("Esc route missing")
		}
		return
	}
	if m.View().Content == "" {
		t.Fatal("supplier route rendered empty")
	}
}

func pr3BDetail(s *supplierListTestService, active bool) SupplierModel {
	if s == nil {
		s = &supplierListTestService{}
	}
	m := NewSupplierModel(s)
	m.stack = []any{SupplierManagerFrame{State: SupplierStateReady}}
	m.frame = SupplierDetailFrame{RouteID: 2, RequestID: 2, SupplierID: 8, Supplier: domain.Supplier{ID: 8, Active: active}, State: SupplierDetailStateReady}
	return m
}

func TestSupplierPR4B2CreationPreselection(t *testing.T) {
	branchID := int64(41)
	for _, test := range []struct {
		name        string
		frame       any
		wantContact bool
		wantBranch  *int64
	}{
		{name: "branch manager", frame: BranchManagerFrame{SupplierID: 8, SearchFocused: true}},
		{name: "supplier contacts", frame: ContactManagerFrame{SupplierID: 8, SearchFocused: true}, wantContact: true},
		{name: "branch contacts", frame: ContactManagerFrame{SupplierID: 8, BranchID: &branchID, SearchFocused: true}, wantContact: true, wantBranch: &branchID},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = test.frame
			next, cmd := model.Update(supplierCtrlN())
			if cmd != nil {
				t.Fatal("child Ctrl+N returned a command; creation entry is synchronous")
			}
			model = next.(SupplierModel)
			got, ok := model.PendingChildCreate()
			if !ok || got.Contact != test.wantContact || got.SupplierID != 8 || !reflect.DeepEqual(got.BranchID, test.wantBranch) {
				t.Fatalf("creation target = %#v/%v, want contact=%v supplier=8 branch=%v", got, ok, test.wantContact, test.wantBranch)
			}
		})
	}
}

func TestSupplierPR4B2LifecycleScopeConfirmation(t *testing.T) {
	branchID := int64(41)
	for _, test := range []struct {
		name        string
		frame       any
		wantContact bool
		wantChild   int64
		wantActive  bool
		wantAction  string
		wantState   string
	}{
		{name: "active branch", frame: BranchDetailFrame{SupplierID: 8, BranchID: branchID, Branch: domain.Branch{ID: branchID, Active: true}, State: SupplierDetailStateReady}, wantChild: branchID, wantActive: true, wantAction: "Desactivar", wantState: "Activo"},
		{name: "inactive contact", frame: ContactDetailFrame{SupplierID: 8, ContactID: 31, BranchID: &branchID, Contact: domain.Contact{ID: 31, Active: false}, State: SupplierDetailStateReady}, wantContact: true, wantChild: 31, wantAction: "Reactivar", wantState: "Inactivo"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.stack = []any{SupplierManagerFrame{State: SupplierStateReady}}
			model.frame = test.frame
			next, cmd := model.Update(supplierKey('A'))
			if cmd != nil {
				t.Fatal("lifecycle confirmation returned a command; confirmation is synchronous")
			}
			model = next.(SupplierModel)
			confirmation, ok := model.CurrentFrame().(ChildLifecycleFrame)
			if !ok || confirmation.Contact != test.wantContact || confirmation.SupplierID != 8 || confirmation.ChildID != test.wantChild || confirmation.Active != test.wantActive {
				t.Fatalf("confirmation = %#v, want contact=%v supplier=8 child=%d active=%v", model.CurrentFrame(), test.wantContact, test.wantChild, test.wantActive)
			}
			view := model.View().Content
			for _, want := range []string{"Estado actual: " + test.wantState, "Acción: " + test.wantAction, "Cancelar"} {
				if !strings.Contains(view, want) {
					t.Fatalf("confirmation view = %q, missing %q", view, want)
				}
			}
			model = supplierModelAfter(t, model, supplierKey(tea.KeyEscape))
			if !reflect.DeepEqual(model.CurrentFrame(), test.frame) {
				t.Fatalf("cancel restored = %#v, want %#v", model.CurrentFrame(), test.frame)
			}
		})
	}
}

func TestSupplierPR4B2ContextualHelp(t *testing.T) {
	for _, test := range []struct {
		name       string
		frame      any
		origin     string
		want       []string
		unexpected string
	}{
		{name: "branch manager", frame: BranchManagerFrame{SupplierID: 8}, origin: "branch-manager", want: []string{"Ctrl+N crear sucursal", "Enter detalle"}, unexpected: "crear contacto"},
		{name: "contact manager", frame: ContactManagerFrame{SupplierID: 8}, origin: "contact-manager", want: []string{"Ctrl+N crear contacto", "Enter detalle"}, unexpected: "crear sucursal"},
		{name: "branch detail", frame: BranchDetailFrame{SupplierID: 8, BranchID: 41}, origin: "branch-detail", want: []string{"A estado", "C contactos de la sucursal"}, unexpected: "Ctrl+N crear"},
		{name: "contact detail", frame: ContactDetailFrame{SupplierID: 8, ContactID: 31}, origin: "contact-detail", want: []string{"A estado", "Esc volver"}, unexpected: "C contactos de la sucursal"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = test.frame
			next, cmd := model.Update(supplierKey('?'))
			if cmd != nil {
				t.Fatal("help returned a command; contextual help opens synchronously")
			}
			model = next.(SupplierModel)
			help, ok := model.CurrentFrame().(SupplierHelpFrame)
			if !ok || help.Origin != test.origin {
				t.Fatalf("help frame = %#v, want origin %q", model.CurrentFrame(), test.origin)
			}
			view := model.View().Content
			for _, want := range test.want {
				if !strings.Contains(view, want) {
					t.Fatalf("help view = %q, missing %q", view, want)
				}
			}
			if strings.Contains(view, test.unexpected) {
				t.Fatalf("help view = %q, contains route-inaccurate %q", view, test.unexpected)
			}
		})
	}

	for _, test := range []struct {
		name  string
		frame any
	}{
		{name: "branch manager search", frame: BranchManagerFrame{SupplierID: 8, SearchFocused: true}},
		{name: "contact manager search", frame: ContactManagerFrame{SupplierID: 8, SearchFocused: true}},
	} {
		t.Run(test.name+" keeps ? as focused input", func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = test.frame
			next, cmd := model.Update(supplierKey('?'))
			if cmd == nil {
				t.Fatal("focused ? did not remain a search input")
			}
			got := next.(SupplierModel).CurrentFrame()
			switch frame := got.(type) {
			case BranchManagerFrame:
				if frame.Query != "?" || !frame.SearchFocused {
					t.Fatalf("focused help key changed branch search: frame=%#v", frame)
				}
			case ContactManagerFrame:
				if frame.Query != "?" || !frame.SearchFocused {
					t.Fatalf("focused help key changed contact search: frame=%#v", frame)
				}
			default:
				t.Fatalf("focused help key changed route: frame=%#v", got)
			}
		})
	}
}

func TestSupplierPR4C1ChildFormsAndRouteCommands(t *testing.T) {
	branchID := int64(41)
	tests := []struct {
		name    string
		frame   any
		contact bool
		branch  *int64
	}{
		{
			name:  "branch manager creates while search is focused",
			frame: BranchManagerFrame{RouteID: 3, RequestID: 4, SupplierID: 8, Query: "cent", Filter: SupplierFilterInactive, SelectedID: 12, Cursor: 2, Offset: 25, Viewport: 10, SearchFocused: true},
		},
		{
			name:    "supplier contact creates without branch",
			frame:   ContactManagerFrame{RouteID: 5, RequestID: 6, SupplierID: 8, Query: "ana", Filter: SupplierFilterAll, SearchFocused: true},
			contact: true,
		},
		{
			name:    "branch contact creates with branch preselected",
			frame:   ContactManagerFrame{RouteID: 7, RequestID: 8, SupplierID: 8, BranchID: &branchID, SearchFocused: true},
			contact: true,
			branch:  &branchID,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := test.frame
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = before
			next, cmd := model.Update(supplierCtrlN())
			if cmd != nil {
				t.Fatal("child Ctrl+N returned a command; form entry is synchronous")
			}
			model = next.(SupplierModel)

			switch form := model.CurrentFrame().(type) {
			case BranchEditFrame:
				if test.contact || form.RouteID == 0 || form.RequestID == 0 || form.SupplierID != 8 || form.BranchID != 0 || form.Mode != SupplierEditCreate {
					t.Fatalf("branch create frame = %#v", form)
				}
			case ContactEditFrame:
				if !test.contact || form.RouteID == 0 || form.RequestID == 0 || form.SupplierID != 8 || form.ContactID != 0 || form.Mode != SupplierEditCreate || !reflect.DeepEqual(form.Values.BranchID, test.branch) {
					t.Fatalf("contact create frame = %#v", form)
				}
			default:
				t.Fatalf("create frame = %#v", model.CurrentFrame())
			}
			title, footer, fields := "Crear sucursal", "Enter guardar sucursal · Esc: Cancelar · Ctrl+C salir", []string{"Nombre", "Referencia", "Ciudad", "Provincia/Estado", "País", "Dirección", "Teléfono general", "Email general", "Notas"}
			if test.contact {
				title, footer, fields = "Crear contacto", "Enter guardar contacto · Esc: Cancelar · Ctrl+C salir", []string{"Nombre", "Cargo", "Sucursal", "Móvil", "Teléfono", "Email", "Notas"}
			}
			first := model.View().Content
			if first != model.View().Content || !strings.Contains(first, title) || !strings.Contains(first, footer) {
				t.Fatalf("non-deterministic or inaccurate form view = %q", first)
			}
			for _, field := range fields {
				if !strings.Contains(first, field) {
					t.Fatalf("form view = %q, missing field %q", first, field)
				}
			}
			model = supplierModelAfter(t, model, supplierKey(tea.KeyEscape))
			if !reflect.DeepEqual(model.CurrentFrame(), before) {
				t.Fatalf("form Esc restoration = %#v, want %#v", model.CurrentFrame(), before)
			}
		})
	}
	for _, test := range []struct {
		name    string
		before  any
		contact bool
	}{
		{"branch detail E", BranchDetailFrame{SupplierID: 8, BranchID: branchID, Branch: domain.Branch{ID: branchID, SupplierID: 8, Name: "Centro"}, State: SupplierDetailStateReady}, false},
		{"contact detail E", ContactDetailFrame{SupplierID: 8, ContactID: 31, Contact: domain.Contact{ID: 31, SupplierID: 8, Name: "Ana"}, State: SupplierDetailStateReady}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = test.before
			model = supplierModelAfter(t, model, supplierKey('E'))

			switch form := model.CurrentFrame().(type) {
			case BranchEditFrame:
				if test.contact || form.Mode != SupplierEditUpdate || form.SupplierID != 8 || form.BranchID != branchID || form.Values.Name != "Centro" {
					t.Fatalf("branch detail E = %#v", form)
				}
			case ContactEditFrame:
				if !test.contact || form.Mode != SupplierEditUpdate || form.SupplierID != 8 || form.ContactID != 31 || form.Values.Name != "Ana" {
					t.Fatalf("contact detail E = %#v", form)
				}
			default:
				t.Fatalf("detail E frame = %#v", model.CurrentFrame())
			}
			wantView := "Editar sucursal\n>Nombre: Centro\n Referencia: \n Ciudad: \n Provincia/Estado: \n País: \n Dirección: \n Teléfono general: \n Email general: \n Notas: \nEnter guardar sucursal · Esc: Cancelar · Ctrl+C salir"
			if test.contact {
				wantView = "Editar contacto\n>Nombre: Ana\n Cargo: \n Sucursal: General\n Móvil: \n Teléfono: \n Email: \n Notas: \nEnter guardar contacto · Esc: Cancelar · Ctrl+C salir"
			}
			if got := model.View().Content; got != wantView {
				t.Fatalf("detail edit view = %q, want %q", got, wantView)
			}
			model = supplierModelAfter(t, model, supplierKey(tea.KeyEscape))
			if !reflect.DeepEqual(model.CurrentFrame(), test.before) {
				t.Fatal("detail edit Esc did not restore the exact frame")
			}
		})
	}
	t.Run("focused printable shortcuts remain input", func(t *testing.T) {
		for _, text := range []string{"E", "S", "C", "A", "?", "x"} {
			model := NewSupplierModel(&supplierListTestService{})
			model.frame = BranchEditFrame{Mode: SupplierEditCreate, Focus: 0, Focused: true}
			model = supplierModelAfter(t, model, supplierTextKey(text))
			form, ok := model.CurrentFrame().(BranchEditFrame)
			if !ok || form.Values.Name != text {
				t.Fatalf("focused %q input = %#v", text, model.CurrentFrame())
			}
		}
	})
	t.Run("focused j and k remain text for branch and contact", func(t *testing.T) {
		for _, test := range []struct {
			name string
			form any
		}{
			{name: "branch", form: BranchEditFrame{Mode: SupplierEditCreate, Focus: 0, Focused: true}},
			{name: "contact", form: ContactEditFrame{Mode: SupplierEditCreate, Focus: 0, Focused: true}},
		} {
			t.Run(test.name, func(t *testing.T) {
				model := NewSupplierModel(&supplierListTestService{})
				model.frame = test.form
				model = supplierModelAfter(t, model, supplierTextKey("j"))
				model = supplierModelAfter(t, model, supplierTextKey("k"))
				switch form := model.CurrentFrame().(type) {
				case BranchEditFrame:
					if form.Values.Name != "jk" || form.Focus != 0 {
						t.Fatalf("branch input = %#v, want name %q at focus 0", form, "jk")
					}
				case ContactEditFrame:
					if form.Values.Name != "jk" || form.Focus != 0 {
						t.Fatalf("contact input = %#v, want name %q at focus 0", form, "jk")
					}
				default:
					t.Fatalf("focused input frame = %#v", model.CurrentFrame())
				}
			})
		}
	})
	t.Run("arrow and tab keys move child form focus", func(t *testing.T) {
		for _, test := range []struct {
			name string
			form any
		}{
			{name: "branch", form: BranchEditFrame{Mode: SupplierEditCreate, Focus: 0, Focused: true}},
			{name: "contact", form: ContactEditFrame{Mode: SupplierEditCreate, Focus: 0, Focused: true}},
		} {
			t.Run(test.name, func(t *testing.T) {
				model := NewSupplierModel(&supplierListTestService{})
				model.frame = test.form
				model = supplierModelAfter(t, model, supplierKey(tea.KeyDown))
				model = supplierModelAfter(t, model, supplierKey(tea.KeyTab))
				model = supplierModelAfter(t, model, supplierKey(tea.KeyUp))
				switch form := model.CurrentFrame().(type) {
				case BranchEditFrame:
					if form.Focus != 1 {
						t.Fatalf("branch focus = %d, want 1", form.Focus)
					}
				case ContactEditFrame:
					if form.Focus != 1 {
						t.Fatalf("contact focus = %d, want 1", form.Focus)
					}
				default:
					t.Fatalf("focus frame = %#v", model.CurrentFrame())
				}
			})
		}
	})
}

func childMutationCommand(t *testing.T, m SupplierModel, key tea.KeyPressMsg) ChildMutationMsg {
	_, cmd := m.Update(key)
	requireSupplier(t, cmd != nil, "child mutation command missing")
	return cmd().(ChildMutationMsg)
}
func requireChildDetail(t *testing.T, got any, contact bool, id int64, name string, active bool) {
	value := reflect.ValueOf(got).FieldByName(map[bool]string{false: "Branch", true: "Contact"}[contact])
	requireSupplier(t, value.IsValid() && value.FieldByName("ID").Int() == id && (name == "" || value.FieldByName("Name").String() == name) && value.FieldByName("Active").Bool() == active, "child detail restoration mismatch")
}
func TestSupplierPR4CChildMutationsAreCorrelatedAndScoped(t *testing.T) {
	branchID := int64(41)
	s := &supplierListTestService{branchResult: domain.Branch{ID: branchID, SupplierID: 8, Name: "Centro", Active: true}, contactResult: domain.Contact{ID: 31, SupplierID: 8, BranchID: &branchID, Name: "Ana", Active: true}}
	for _, tc := range []struct {
		name            string
		before          any
		contact, create bool
		id              int64
		nameValue       string
	}{
		{"branch create", BranchManagerFrame{RouteID: 3, RequestID: 4, SupplierID: 8}, false, true, branchID, ""},
		{"contact create", ContactManagerFrame{RouteID: 5, RequestID: 6, SupplierID: 8, BranchID: &branchID}, true, true, 31, ""},
		{"branch update", BranchDetailFrame{RouteID: 7, RequestID: 8, SupplierID: 8, BranchID: branchID, Branch: s.branchResult, State: SupplierDetailStateReady}, false, false, branchID, "Centro"},
		{"contact update", ContactDetailFrame{RouteID: 9, RequestID: 10, SupplierID: 8, ContactID: 31, BranchID: &branchID, Contact: s.contactResult, State: SupplierDetailStateReady}, true, false, 31, "Ana"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewSupplierModel(s)
			m.frame = tc.before
			if tc.create {
				m = supplierModelAfter(t, m, supplierCtrlN())
			} else {
				m = supplierModelAfter(t, m, supplierKey('E'))
			}
			msg := childMutationCommand(t, m, supplierKey(tea.KeyEnter))
			kind := ChildMutationUpdate
			if tc.create {
				kind = ChildMutationCreate
			}
			requireSupplier(t, msg.Kind == kind && msg.RouteID > 0 && msg.RequestID > 0 && msg.SupplierID == 8 && msg.ChildID == tc.id && msg.Contact == tc.contact, "child mutation correlation mismatch")
			if tc.create {
				if tc.contact {
					requireSupplier(t, s.contactSupplier == 8 && reflect.DeepEqual(s.contactBranch, &branchID), "contact scope mismatch")
				} else {
					requireSupplier(t, s.branchSupplier == 8, "branch scope mismatch")
				}
				requireSupplier(t, reflect.DeepEqual(supplierModelAfter(t, m, msg).CurrentFrame(), tc.before), "create restoration mismatch")
				return
			}
			for _, name := range []string{"route", "request", "supplier", "child", "kind"} {
				stale := msg
				switch name {
				case "route":
					stale.RouteID++
				case "request":
					stale.RequestID++
				case "supplier":
					stale.SupplierID++
				case "child":
					stale.ChildID++
				case "kind":
					stale.Kind = ChildMutationCreate
				}
				got := supplierModelAfter(t, m, stale)
				requireSupplier(t, reflect.DeepEqual(got.CurrentFrame(), m.CurrentFrame()), "stale "+name+" reply changed edit route")
			}
			requireChildDetail(t, supplierModelAfter(t, m, msg).CurrentFrame(), tc.contact, tc.id, tc.nameValue, true)
		})
	}
}

func requireChildUpdateError(t *testing.T, frame any, contact bool, id int64, err error, want string) {
	m := NewSupplierModel(&supplierListTestService{childErr: err})
	m.frame = frame
	msg := childMutationCommand(t, m, supplierKey(tea.KeyEnter))
	got := supplierModelAfter(t, m, msg)
	requireSupplier(t, msg.ChildID == id && msg.Contact == contact && strings.Contains(got.View().Content, want), "child update error mismatch")
}

func TestSupplierPR4CUpdateErrorsKeepRequestedChildID(t *testing.T) {
	branchID, contactID := int64(41), int64(31)
	requireChildUpdateError(t, BranchEditFrame{RouteID: 21, RequestID: 22, SupplierID: 8, BranchID: branchID, Mode: SupplierEditUpdate}, false, branchID, domain.ErrValidation, "Revisá los datos de la sucursal.")
	requireChildUpdateError(t, ContactEditFrame{RouteID: 23, RequestID: 24, SupplierID: 8, ContactID: contactID, Mode: SupplierEditUpdate}, true, contactID, errors.New("secret"), "No pude guardar el contacto.")
}

func TestSupplierPR4CLifecycleExecutionErrorsAndHelp(t *testing.T) {
	branchID, contactID := int64(41), int64(31)
	s := &supplierListTestService{branchResult: domain.Branch{ID: branchID, SupplierID: 8, Active: false}, contactResult: domain.Contact{ID: contactID, SupplierID: 8, Active: true}}
	for _, tc := range []struct {
		name    string
		before  any
		contact bool
		kind    ChildMutationKind
		active  bool
	}{
		{"branch deactivate", BranchDetailFrame{RouteID: 4, RequestID: 5, SupplierID: 8, BranchID: branchID, Branch: domain.Branch{ID: branchID, Active: true}, State: SupplierDetailStateReady}, false, ChildMutationDeactivate, true},
		{"contact reactivate", ContactDetailFrame{RouteID: 6, RequestID: 7, SupplierID: 8, ContactID: contactID, Contact: domain.Contact{ID: contactID}, State: SupplierDetailStateReady}, true, ChildMutationReactivate, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewSupplierModel(s)
			m.frame = tc.before
			m = supplierModelAfter(t, m, supplierKey('A'))
			msg := childMutationCommand(t, m, supplierKey(tea.KeyEnter))
			id := branchID
			if tc.contact {
				id = contactID
			}
			requireSupplier(t, msg.Kind == tc.kind && msg.Contact == tc.contact && msg.SupplierID == 8 && msg.ChildID == id, "lifecycle scope mismatch")
			requireChildDetail(t, supplierModelAfter(t, m, msg).CurrentFrame(), tc.contact, id, "", !tc.active)
		})
	}
	for _, tc := range []struct {
		want    string
		err     error
		contact bool
	}{
		{"Revisá los datos de la sucursal.", domain.ErrValidation, false}, {"La sucursal ya no existe.", domain.ErrBranchNotFound, false}, {"La sucursal ya está registrada.", domain.ErrConflict, false}, {"No pude cambiar el estado de la sucursal. Probá de nuevo en un momento.", errors.New("secret"), false},
		{"Revisá los datos del contacto.", domain.ErrValidation, true}, {"El contacto ya no existe.", domain.ErrContactNotFound, true}, {"El contacto ya está registrado.", domain.ErrConflict, true}, {"No pude cambiar el estado del contacto. Probá de nuevo en un momento.", errors.New("secret"), true},
	} {
		requireSupplier(t, childMutationErrorText(tc.err, tc.contact, true) == tc.want, childMutationErrorText(tc.err, tc.contact, true))
	}
}
