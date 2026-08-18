package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
)

type BranchManagerFrame struct {
	RouteID, RequestID       uint64
	SupplierID               int64
	Query                    string
	Filter                   SupplierFilter
	Items                    []SupplierDetailItem
	SelectedID               int64
	Cursor, Offset, Viewport int
	State                    SupplierState
	Error                    string
	SearchFocused            bool
}
type ContactManagerFrame struct {
	RouteID, RequestID       uint64
	SupplierID               int64
	BranchID                 *int64
	Query                    string
	Filter                   SupplierFilter
	Items                    []SupplierDetailItem
	SelectedID               int64
	Cursor, Offset, Viewport int
	State                    SupplierState
	Error                    string
	SearchFocused            bool
	BranchNames              map[int64]string
}
type BranchListMsg struct {
	RouteID, RequestID uint64
	Branches           []domain.Branch
	Err                error
}
type ContactListMsg struct {
	RouteID, RequestID uint64
	Contacts           []domain.Contact
	Err                error
}

func branchItems(values []domain.Branch) []SupplierDetailItem {
	items := make([]SupplierDetailItem, 0, len(values)+1)
	city := ""
	for _, branch := range values {
		if branch.City != city {
			city = branch.City
			items = append(items, SupplierDetailItem{Label: city})
		}
		items = append(items, SupplierDetailItem{Kind: SupplierDetailBranch, Label: branch.Name, Selectable: true, Target: SupplierNavigationTarget{SupplierID: branch.SupplierID, BranchID: branch.ID}})
	}
	return items
}
func contactItems(values []domain.Contact, names map[int64]string) []SupplierDetailItem {
	items := make([]SupplierDetailItem, 0, len(values)+1)
	heading := ""
	for _, contact := range values {
		name := "General"
		if contact.BranchID != nil {
			name = names[*contact.BranchID]
		}
		if name != heading {
			heading = name
			items = append(items, SupplierDetailItem{Label: name})
		}
		items = append(items, supplierContactItem(contact))
	}
	return items
}
func branchListCmd(service SupplierDetailService, frame BranchManagerFrame) tea.Cmd {
	return func() tea.Msg {
		values, err := service.ListBranches(context.Background(), frame.SupplierID, domain.ListCriteria{Text: frame.Query, Active: supplierFilterActive(frame.Filter), Limit: supplierPageSize, Offset: frame.Offset})
		return BranchListMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Branches: values, Err: err}
	}
}
func contactListCmd(service SupplierDetailService, frame ContactManagerFrame) tea.Cmd {
	return func() tea.Msg {
		values, err := service.ListContacts(context.Background(), frame.SupplierID, domain.ContactListCriteria{Text: frame.Query, Active: supplierFilterActive(frame.Filter), BranchID: frame.BranchID, Limit: supplierPageSize, Offset: frame.Offset})
		return ContactListMsg{RouteID: frame.RouteID, RequestID: frame.RequestID, Contacts: values, Err: err}
	}
}
func childResult(err error, empty bool, name string) (SupplierState, string) {
	if err != nil {
		return SupplierStateError, "No pude cargar " + name + ". Probá de nuevo en un momento."
	}
	if empty {
		return SupplierStateEmpty, ""
	}
	return SupplierStateReady, ""
}
func firstChild(items []SupplierDetailItem, contact bool) (int, int64) {
	for i, item := range items {
		if item.Selectable {
			if contact {
				return i, item.Target.ContactID
			}
			return i, item.Target.BranchID
		}
	}
	return 0, 0
}
func (m SupplierModel) InitChild() tea.Cmd {
	service, ok := m.service.(SupplierDetailService)
	if !ok {
		return nil
	}
	switch frame := m.frame.(type) {
	case BranchManagerFrame:
		return branchListCmd(service, frame)
	case ContactManagerFrame:
		return contactListCmd(service, frame)
	}
	return nil
}
func (m SupplierModel) openBranchManager(previous any, supplierID int64) (SupplierModel, tea.Cmd) {
	m.pushFrame(previous)
	m.frame = BranchManagerFrame{RouteID: m.nextRouteID, RequestID: m.nextRequestID, SupplierID: supplierID, Filter: SupplierFilterActive, State: SupplierStateLoading, Viewport: 10}
	return m, m.InitChild()
}
func (m SupplierModel) openContactManager(previous any, supplierID int64, branchID *int64) (SupplierModel, tea.Cmd) {
	var id *int64
	if branchID != nil {
		value := *branchID
		id = &value
	}
	names := map[int64]string{}
	if detail, ok := previous.(SupplierDetailFrame); ok {
		city := ""
		for _, item := range detail.Items {
			switch item.Kind {
			case SupplierDetailHeading:
				city = item.Label
			case SupplierDetailBranch:
				names[item.Target.BranchID] = city + " / " + item.Label
			}
		}
	}
	m.pushFrame(previous)
	m.frame = ContactManagerFrame{RouteID: m.nextRouteID, RequestID: m.nextRequestID, SupplierID: supplierID, BranchID: id, Filter: SupplierFilterActive, State: SupplierStateLoading, Viewport: 10, BranchNames: names}
	return m, m.InitChild()
}
func (m SupplierModel) childListReply(msg tea.Msg) SupplierModel {
	switch value := msg.(type) {
	case BranchListMsg:
		frame, ok := m.frame.(BranchManagerFrame)
		if !ok || frame.RouteID != value.RouteID || frame.RequestID != value.RequestID {
			return m
		}
		frame.Items = branchItems(value.Branches)
		frame.State, frame.Error = childResult(value.Err, len(value.Branches) == 0, "las sucursales")
		frame.Cursor, frame.SelectedID = firstChild(frame.Items, false)
		m.frame = frame
	case ContactListMsg:
		frame, ok := m.frame.(ContactManagerFrame)
		if !ok || frame.RouteID != value.RouteID || frame.RequestID != value.RequestID {
			return m
		}
		frame.Items = contactItems(value.Contacts, frame.BranchNames)
		frame.State, frame.Error = childResult(value.Err, len(value.Contacts) == 0, "los contactos")
		frame.Cursor, frame.SelectedID = firstChild(frame.Items, true)
		m.frame = frame
	}
	return m
}
func childManagerView(title, state string, labels []string) string {
	lines := []string{title, state}
	lines = append(lines, labels...)
	lines = append(lines, "Ctrl+N crear · Enter detalle · E buscar · Esc volver · ? ayuda · Ctrl+C salir")
	return strings.Join(lines, "\n")
}
func childLabels[T any](items []T, label func(T) string) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = label(item)
	}
	return labels
}
func (m SupplierModel) branchManagerView(f BranchManagerFrame) string {
	return childManagerView("Sucursales", f.State.text(f.Error), childLabels(f.Items, func(item SupplierDetailItem) string { return item.Label }))
}
func (m SupplierModel) contactManagerView(f ContactManagerFrame) string {
	return childManagerView("Contactos", f.State.text(f.Error), childLabels(f.Items, func(item SupplierDetailItem) string { return item.Label }))
}
