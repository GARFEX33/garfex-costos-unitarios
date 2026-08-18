package tui

import (
	"errors"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
)

type SupplierModel struct {
	service       SupplierListService
	frame         any
	stack         []any
	nextRouteID   uint64
	nextRequestID uint64
	navigation    *SupplierNavigationTarget
	childCreate   *ChildCreateTarget
	AtRoot        bool
}

var supplierKeyDelta = map[string]int{"up": -1, "k": -1, "down": 1, "j": 1}

func NewSupplierModel(service SupplierListService) SupplierModel {
	return SupplierModel{
		service:       service,
		frame:         SupplierManagerFrame{RouteID: 1, RequestID: 1, Filter: SupplierFilterActive, State: SupplierStateLoading, Viewport: 10},
		nextRouteID:   1,
		nextRequestID: 1,
	}
}

func (m SupplierModel) Init() tea.Cmd {
	return supplierListCmd(m.service, m.frame.(SupplierManagerFrame))
}

func (m SupplierModel) CurrentFrame() any { return m.frame }

func (m SupplierModel) View() tea.View { return tea.NewView(m.supplierView()) }

func (m SupplierModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case SupplierListMsg:
		return m.listReply(value), nil
	case SupplierDetailMsg:
		return m.detailReply(value), nil
	case BranchDetailMsg:
		return m.branchDetailReply(value), nil
	case ContactDetailMsg:
		return m.contactDetailReply(value), nil
	case BranchListMsg, ContactListMsg:
		return m.childListReply(value), nil
	case SupplierMutationMsg:
		return m.mutationReply(value)
	case ChildMutationMsg:
		return m.childMutationReply(value)
	case tea.KeyPressMsg:
		return m.updateKey(value)
	default:
		return m, nil
	}
}

func (m SupplierModel) listReply(msg SupplierListMsg) SupplierModel {
	frame, ok := m.frame.(SupplierManagerFrame)
	if !ok || frame.RouteID != msg.RouteID || frame.RequestID != msg.RequestID {
		return m
	}
	frame.Rows = msg.Rows
	frame.State, frame.Error = supplierResultState(msg.Err, len(msg.Rows) == 0)
	if frame.SelectedID == 0 && len(msg.Rows) > 0 {
		frame.SelectedID = msg.Rows[0].ID
		frame.Cursor = 0
	}
	m.frame = frame
	return m
}

func (m SupplierModel) detailReply(msg SupplierDetailMsg) SupplierModel {
	frame, ok := m.frame.(SupplierDetailFrame)
	if !ok || frame.RouteID != msg.RouteID || frame.RequestID != msg.RequestID {
		return m
	}
	frame.Supplier = msg.Detail.Supplier
	frame.Items = supplierDetailItems(msg.Detail)
	if msg.Err != nil {
		frame.State, frame.Error = SupplierDetailStateError, "No pude cargar el detalle del proveedor. Probá de nuevo en un momento."
	} else if len(frame.Items) == 0 && frame.Supplier.ID == 0 {
		frame.State, frame.Error = SupplierDetailStateEmpty, ""
	} else {
		frame.State, frame.Error = SupplierDetailStateReady, ""
	}
	m.frame = frame
	return m
}

func (m SupplierModel) branchDetailReply(msg BranchDetailMsg) SupplierModel {
	frame, ok := m.frame.(BranchDetailFrame)
	if !ok || frame.RouteID != msg.RouteID || frame.RequestID != msg.RequestID {
		return m
	}
	frame.Supplier, frame.Branch, frame.Contacts = msg.Supplier, msg.Branch, msg.Contacts
	frame.State, frame.Error = childDetailResult(msg.Err, frame.Branch.ID == 0, "de la sucursal")
	m.frame = frame
	return m
}

func (m SupplierModel) contactDetailReply(msg ContactDetailMsg) SupplierModel {
	frame, ok := m.frame.(ContactDetailFrame)
	if !ok || frame.RouteID != msg.RouteID || frame.RequestID != msg.RequestID {
		return m
	}
	frame.Contact, frame.Branch, frame.BranchID = msg.Contact, msg.Branch, msg.Contact.BranchID
	frame.State, frame.Error = childDetailResult(msg.Err, frame.Contact.ID == 0, "del contacto")
	m.frame = frame
	return m
}

func childDetailResult(err error, empty bool, name string) (SupplierDetailState, string) {
	if err != nil {
		return SupplierDetailStateError, "No pude cargar el detalle " + name + ". Probá de nuevo en un momento."
	}
	if empty {
		return SupplierDetailStateEmpty, ""
	}
	return SupplierDetailStateReady, ""
}

func supplierResultState(err error, empty bool) (SupplierState, string) {
	if err != nil {
		return SupplierStateError, "No pude cargar los proveedores. Probá de nuevo en un momento."
	}
	if empty {
		return SupplierStateEmpty, ""
	}
	return SupplierStateReady, ""
}

func supplierMutationErrorText(err error, lifecycle bool) string {
	switch {
	case errors.Is(err, domain.ErrValidation):
		return "Revisá los datos del proveedor."
	case errors.Is(err, domain.ErrNotFound):
		return "El proveedor ya no existe."
	case errors.Is(err, domain.ErrConflict):
		return "El identificador fiscal ya está registrado."
	case lifecycle:
		return "No pude cambiar el estado del proveedor. Probá de nuevo en un momento."
	default:
		return "No pude guardar el proveedor. Probá de nuevo en un momento."
	}
}

func (m SupplierModel) LoadSuppliers(query string, filter SupplierFilter) (SupplierModel, tea.Cmd) {
	frame, ok := m.frame.(SupplierManagerFrame)
	if !ok {
		return m, nil
	}
	frame.Query, frame.Filter = query, filter
	frame.Rows, frame.SelectedID, frame.Cursor, frame.Offset = nil, 0, 0, 0
	m.nextRequestID++
	frame.RequestID = m.nextRequestID
	frame.State, frame.Error = SupplierStateLoading, ""
	m.frame = frame
	return m, supplierListCmd(m.service, frame)
}

func (m SupplierModel) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, ok := m.updateChildEdit(msg); ok {
		return next, cmd
	}
	if delta, ok := supplierKeyDelta[msg.String()]; ok {
		switch frame := m.frame.(type) {
		case SupplierManagerFrame:
			next := frame.Cursor + delta
			if next >= 0 && next < len(frame.Rows) {
				frame.Cursor = next
				frame.SelectedID = frame.Rows[next].ID
			}
			m.frame = frame
		case SupplierDetailFrame:
			frame.Cursor = detailCursor(frame.Items, frame.Cursor, delta)
			m.frame = frame
		case SupplierEditFrame:
			next := frame.Focus + delta
			if next >= 0 && next < len(supplierFormFields) {
				frame.Focus = next
			}
			m.frame = frame
		case BranchManagerFrame:
			frame.Cursor = detailCursor(frame.Items, frame.Cursor, delta)
			if frame.Cursor >= 0 && frame.Cursor < len(frame.Items) {
				frame.SelectedID = frame.Items[frame.Cursor].Target.BranchID
			}
			m.frame = frame
		case ContactManagerFrame:
			frame.Cursor = detailCursor(frame.Items, frame.Cursor, delta)
			if frame.Cursor >= 0 && frame.Cursor < len(frame.Items) {
				frame.SelectedID = frame.Items[frame.Cursor].Target.ContactID
			}
			m.frame = frame
		}
		return m, nil
	}
	switch frame := m.frame.(type) {
	case SupplierManagerFrame:
		if msg.String() == "ctrl+n" {
			return m.openEdit(frame, domain.Supplier{}, SupplierEditCreate)
		}
		if msg.String() == "enter" {
			return m.openDetail(frame.SelectedID)
		}
		if msg.String() == "esc" {
			m.AtRoot = true
		}
		if msg.String() == "?" && !frame.SearchFocused {
			return m.openHelp(frame, "manager")
		}
	case SupplierDetailFrame:
		if msg.String() == "s" || msg.String() == "S" {
			return m.openBranchManager(frame, frame.SupplierID)
		}
		if msg.String() == "c" || msg.String() == "C" {
			return m.openContactManager(frame, frame.SupplierID, nil)
		}
		if msg.String() == "e" || msg.String() == "E" {
			supplier := frame.Supplier
			if supplier.ID == 0 {
				supplier.ID = frame.SupplierID
			}
			return m.openEdit(frame, supplier, SupplierEditUpdate)
		}
		if msg.String() == "enter" && frame.Cursor >= 0 && frame.Cursor < len(frame.Items) && frame.Items[frame.Cursor].Selectable {
			item := frame.Items[frame.Cursor]
			target := item.Target
			m.navigation = &target
		}
		if msg.String() == "esc" && len(m.stack) > 0 {
			return m.popFrame()
		}
		if msg.String() == "a" || msg.String() == "A" {
			return m.openLifecycle(frame)
		}
		if msg.String() == "?" {
			return m.openHelp(frame, "detail")
		}
	case BranchManagerFrame:
		if msg.String() == "enter" && frame.SelectedID > 0 {
			return m.openChildDetail(frame.SelectedID)
		}
		if msg.String() == "ctrl+n" {
			m.childCreate = &ChildCreateTarget{SupplierID: frame.SupplierID}
			return m.openBranchEdit(frame, domain.Branch{SupplierID: frame.SupplierID}, SupplierEditCreate)
		}
		if msg.String() == "?" && !frame.SearchFocused {
			return m.openHelp(frame, "branch-manager")
		}
		if msg.String() == "esc" {
			return m.popFrame()
		}
		if frame.SearchFocused {
			if text := supplierPrintableText(msg); text != "" {
				frame.Query += text
				frame.Offset, frame.Cursor, frame.SelectedID = 0, 0, 0
				m.nextRequestID++
				frame.RequestID = m.nextRequestID
				frame.State = SupplierStateLoading
				m.frame = frame
				return m, branchListCmd(m.service.(SupplierDetailService), frame)
			}
		}
	case ContactManagerFrame:
		if msg.String() == "enter" && frame.SelectedID > 0 {
			return m.openChildDetail(frame.SelectedID)
		}
		if msg.String() == "ctrl+n" {
			target := &ChildCreateTarget{Contact: true, SupplierID: frame.SupplierID}
			if frame.BranchID != nil {
				value := *frame.BranchID
				target.BranchID = &value
			}
			m.childCreate = target
			return m.openContactEdit(frame, domain.Contact{SupplierID: frame.SupplierID, BranchID: target.BranchID}, SupplierEditCreate)
		}
		if msg.String() == "?" && !frame.SearchFocused {
			return m.openHelp(frame, "contact-manager")
		}
		if msg.String() == "esc" {
			return m.popFrame()
		}
		if frame.SearchFocused {
			if text := supplierPrintableText(msg); text != "" {
				frame.Query += text
				frame.Offset, frame.Cursor, frame.SelectedID = 0, 0, 0
				m.nextRequestID++
				frame.RequestID = m.nextRequestID
				frame.State = SupplierStateLoading
				m.frame = frame
				return m, contactListCmd(m.service.(SupplierDetailService), frame)
			}
		}
	case BranchDetailFrame:
		if msg.String() == "e" || msg.String() == "E" {
			return m.openBranchEdit(frame, frame.Branch, SupplierEditUpdate)
		}
		if msg.String() == "c" || msg.String() == "C" {
			return m.openContactManager(frame, frame.SupplierID, &frame.BranchID)
		}
		if msg.String() == "a" || msg.String() == "A" {
			return m.openChildLifecycle(frame, false, frame.BranchID, frame.Branch.Active)
		}
		if msg.String() == "?" {
			return m.openHelp(frame, "branch-detail")
		}
		if msg.String() == "esc" {
			return m.popFrame()
		}
	case ContactDetailFrame:
		if msg.String() == "e" || msg.String() == "E" {
			return m.openContactEdit(frame, frame.Contact, SupplierEditUpdate)
		}
		if msg.String() == "a" || msg.String() == "A" {
			return m.openChildLifecycle(frame, true, frame.ContactID, frame.Contact.Active)
		}
		if msg.String() == "?" {
			return m.openHelp(frame, "contact-detail")
		}
		if msg.String() == "esc" {
			return m.popFrame()
		}
	case ChildLifecycleFrame:
		if msg.String() == "enter" {
			return m, childMutationCmd(m.service, frame)
		}
		if msg.String() == "esc" || msg.String() == "c" || msg.String() == "C" {
			return m.popFrame()
		}
	case SupplierEditFrame:
		if msg.String() == "tab" {
			frame.Focus = (frame.Focus + 1) % len(supplierFormFields)
			m.frame = frame
			return m, nil
		}
		if text := supplierPrintableText(msg); frame.Focused && text != "" {
			frame = appendSupplierText(frame, text)
			m.frame = frame
			return m, nil
		}
		if msg.String() == "esc" {
			return m.popFrame()
		}
		if msg.String() == "enter" {
			if frame.Mode {
				service, ok := m.service.(SupplierUpdateService)
				if !ok {
					frame.Error = "No pude guardar el proveedor. Probá de nuevo en un momento."
					m.frame = frame
					return m, nil
				}
				return m, supplierUpdateCmd(service, frame)
			}
			service, ok := m.service.(SupplierCreateService)
			if !ok {
				frame.Error = "No pude guardar el proveedor. Probá de nuevo en un momento."
				m.frame = frame
				return m, nil
			}
			return m, supplierCreateCmd(service, frame)
		}
		if msg.String() == "?" && !frame.Focused {
			return m.openHelp(frame, "edit")
		}
	case SupplierLifecycleFrame:
		if msg.String() == "esc" || msg.String() == "c" || msg.String() == "C" {
			return m.popFrame()
		}
		if msg.String() == "enter" {
			service, ok := m.service.(SupplierLifecycleService)
			if !ok {
				frame.Error = supplierMutationErrorText(errors.New("lifecycle service unavailable"), true)
				m.frame = frame
				return m, nil
			}
			return m, supplierLifecycleCmd(service, frame)
		}
	case BranchEditFrame:
		if msg.String() == "enter" {
			return m, childMutationCmd(m.service, frame)
		}
	case ContactEditFrame:
		if msg.String() == "enter" {
			return m, childMutationCmd(m.service, frame)
		}
	case SupplierHelpFrame:
		if msg.String() == "esc" {
			return m.popFrame()
		}
	}
	return m, nil
}

func (m SupplierModel) mutationReply(msg SupplierMutationMsg) (SupplierModel, tea.Cmd) {
	switch frame := m.frame.(type) {
	case SupplierEditFrame:
		if frame.RouteID != msg.RouteID || frame.RequestID != msg.RequestID {
			return m, nil
		}
		if msg.Err != nil {
			frame.Error = supplierMutationErrorText(msg.Err, false)
			m.frame = frame
			return m, nil
		}
		previous, ok := m.previousFrame()
		if !ok {
			return m, nil
		}
		if frame.Mode {
			m.frame = previous
			m.stack = m.stack[:len(m.stack)-1]
			if detail, ok := previous.(SupplierDetailFrame); ok {
				detail.Supplier = msg.Supplier
				detail.State, detail.Error = SupplierDetailStateReady, ""
				m.frame = detail
			}
			return m, nil
		}
		manager, ok := previous.(SupplierManagerFrame)
		if !ok {
			return m, nil
		}
		manager.Rows, manager.SelectedID, manager.Cursor = nil, 0, 0
		manager.State, manager.Error = SupplierStateLoading, ""
		m.frame = manager
		m.stack = m.stack[:len(m.stack)-1]
		m.nextRequestID++
		manager.RequestID = m.nextRequestID
		m.frame = manager
		return m, supplierListCmd(m.service, manager)
	case SupplierLifecycleFrame:
		if frame.RouteID != msg.RouteID || frame.RequestID != msg.RequestID {
			return m, nil
		}
		if msg.Err != nil {
			frame.Error = supplierMutationErrorText(msg.Err, true)
			m.frame = frame
			return m, nil
		}
		previous, ok := m.previousFrame()
		if !ok {
			return m, nil
		}
		detail, ok := previous.(SupplierDetailFrame)
		if !ok {
			return m, nil
		}
		if msg.Supplier.ID == 0 {
			msg.Supplier = frame.Supplier
			msg.Supplier.Active = !frame.Active
		}
		detail.Supplier = msg.Supplier
		detail.State, detail.Error = SupplierDetailStateReady, ""
		m.frame = detail
		m.stack = m.stack[:len(m.stack)-1]
		return m, nil
	}
	return m, nil
}

func (m SupplierModel) childMutationReply(msg ChildMutationMsg) (SupplierModel, tea.Cmd) {
	if !childMutationMatches(m.frame, msg) {
		return m, nil
	}
	if msg.Err != nil {
		switch frame := m.frame.(type) {
		case BranchEditFrame:
			frame.Error = childMutationErrorText(msg.Err, false, false)
			m.frame = frame
		case ContactEditFrame:
			frame.Error = childMutationErrorText(msg.Err, true, false)
			m.frame = frame
		case ChildLifecycleFrame:
			frame.Error = childMutationErrorText(msg.Err, frame.Contact, true)
			m.frame = frame
		}
		return m, nil
	}
	previous, ok := m.previousFrame()
	if !ok {
		return m, nil
	}
	switch frame := previous.(type) {
	case BranchDetailFrame:
		if msg.Branch.ID == 0 {
			msg.Branch = frame.Branch
			msg.Branch.Active = !frame.Branch.Active
		}
		frame.Branch = msg.Branch
		frame.State, frame.Error = SupplierDetailStateReady, ""
		m.frame = frame
	case ContactDetailFrame:
		if msg.ContactValue.ID == 0 {
			msg.ContactValue = frame.Contact
			msg.ContactValue.Active = !frame.Contact.Active
		}
		frame.Contact, frame.BranchID = msg.ContactValue, msg.ContactValue.BranchID
		frame.State, frame.Error = SupplierDetailStateReady, ""
		m.frame = frame
	default:
		m.frame = previous
	}
	m.stack = m.stack[:len(m.stack)-1]
	return m, nil
}

func childMutationMatches(frame any, msg ChildMutationMsg) bool {
	route, request, contact, supplierID, childID := childMutationIdentity(frame)
	if route == 0 || route != msg.RouteID || request != msg.RequestID || contact != msg.Contact || supplierID != msg.SupplierID {
		return false
	}
	if childID == 0 {
		return msg.Kind == ChildMutationCreate
	}
	if msg.ChildID != childID {
		return false
	}
	switch value := frame.(type) {
	case BranchEditFrame, ContactEditFrame:
		return msg.Kind == ChildMutationUpdate
	case ChildLifecycleFrame:
		return msg.Kind == map[bool]ChildMutationKind{true: ChildMutationDeactivate, false: ChildMutationReactivate}[value.Active]
	default:
		return false
	}
}

func childMutationErrorText(err error, contact, lifecycle bool) string {
	entity, genitive, title, registered := "la sucursal", "de la sucursal", "La sucursal", "registrada"
	if contact {
		entity, genitive, title, registered = "el contacto", "del contacto", "El contacto", "registrado"
	}
	switch {
	case errors.Is(err, domain.ErrValidation):
		return "Revisá los datos " + genitive + "."
	case errors.Is(err, domain.ErrNotFound):
		return title + " ya no existe."
	case errors.Is(err, domain.ErrConflict):
		return title + " ya está " + registered + "."
	case lifecycle:
		return "No pude cambiar el estado " + genitive + ". Probá de nuevo en un momento."
	default:
		return "No pude guardar " + entity + ". Probá de nuevo en un momento."
	}
}

func (m SupplierModel) previousFrame() (any, bool) {
	if len(m.stack) == 0 {
		return nil, false
	}
	return m.stack[len(m.stack)-1], true
}

func supplierPrintableText(msg tea.KeyPressMsg) string {
	if msg.Key().Mod != 0 {
		return ""
	}
	if msg.Key().Text != "" {
		return msg.Key().Text
	}
	if unicode.IsPrint(msg.Key().Code) {
		return string(msg.Key().Code)
	}
	return ""
}

func appendSupplierText(frame SupplierEditFrame, text string) SupplierEditFrame {
	switch frame.Focus {
	case SupplierFieldTradeName:
		frame.Values.TradeName += text
	case SupplierFieldLegalName:
		frame.Values.LegalName += text
	case SupplierFieldTaxIdentifier:
		frame.Values.TaxIdentifier += text
	case SupplierFieldWebsite:
		frame.Values.Website += text
	default:
		frame.Values.Notes += text
	}
	return frame
}

func (m SupplierModel) openEdit(previous any, supplier domain.Supplier, mode bool) (SupplierModel, tea.Cmd) {
	if mode && supplier.ID <= 0 {
		return m, nil
	}
	m.pushFrame(previous)
	m.frame = newSupplierEditFrame(m.nextRouteID, m.nextRequestID, supplier, mode)
	return m, nil
}

func (m *SupplierModel) pushFrame(frame any) {
	m.stack = append(m.stack, frame)
	m.nextRouteID++
	m.nextRequestID++
}

func (m SupplierModel) popFrame() (SupplierModel, tea.Cmd) {
	if len(m.stack) == 0 {
		return m, nil
	}
	m.frame = m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.navigation = nil
	return m, nil
}

func (m SupplierModel) openLifecycle(previous SupplierDetailFrame) (SupplierModel, tea.Cmd) {
	m.pushFrame(previous)
	m.frame = SupplierLifecycleFrame{
		RouteID: m.nextRouteID, RequestID: m.nextRequestID,
		SupplierID: previous.SupplierID, Supplier: previous.Supplier, Active: previous.Supplier.Active,
	}
	return m, nil
}

func (m SupplierModel) openHelp(previous any, origin string) (SupplierModel, tea.Cmd) {
	m.pushFrame(previous)
	m.frame = SupplierHelpFrame{RouteID: m.nextRouteID, RequestID: m.nextRequestID, Origin: origin}
	return m, nil
}

func (m SupplierModel) openDetail(id int64) (SupplierModel, tea.Cmd) {
	if id <= 0 {
		return m, nil
	}
	service, ok := m.service.(SupplierDetailService)
	if !ok {
		return m, nil
	}
	manager, ok := m.frame.(SupplierManagerFrame)
	if !ok {
		return m, nil
	}
	m.stack = append(m.stack, manager)
	m.nextRouteID++
	m.nextRequestID++
	detail := SupplierDetailFrame{RouteID: m.nextRouteID, RequestID: m.nextRequestID, SupplierID: id, State: SupplierDetailStateLoading}
	m.frame, m.navigation = detail, nil
	return m, supplierDetailCmd(service, detail)
}

func (m SupplierModel) openChildDetail(id int64) (SupplierModel, tea.Cmd) {
	service, ok := m.service.(SupplierDetailService)
	if !ok || id <= 0 {
		return m, nil
	}
	switch frame := m.frame.(type) {
	case BranchManagerFrame:
		m.pushFrame(frame)
		detail := BranchDetailFrame{RouteID: m.nextRouteID, RequestID: m.nextRequestID, SupplierID: frame.SupplierID, BranchID: id, State: SupplierDetailStateLoading}
		m.frame = detail
		return m, branchDetailCmd(service, detail)
	case ContactManagerFrame:
		m.pushFrame(frame)
		detail := ContactDetailFrame{RouteID: m.nextRouteID, RequestID: m.nextRequestID, SupplierID: frame.SupplierID, ContactID: id, BranchID: frame.BranchID, State: SupplierDetailStateLoading}
		m.frame = detail
		return m, contactDetailCmd(service, detail)
	default:
		return m, nil
	}
}

func (m SupplierModel) PendingChildCreate() (ChildCreateTarget, bool) {
	if m.childCreate == nil {
		return ChildCreateTarget{}, false
	}
	return *m.childCreate, true
}

func (m SupplierModel) openChildLifecycle(previous any, contact bool, id int64, active bool) (SupplierModel, tea.Cmd) {
	m.pushFrame(previous)
	supplierID := int64(0)
	switch frame := previous.(type) {
	case BranchDetailFrame:
		supplierID = frame.SupplierID
	case ContactDetailFrame:
		supplierID = frame.SupplierID
	}
	m.frame = ChildLifecycleFrame{RouteID: m.nextRouteID, RequestID: m.nextRequestID, Contact: contact, SupplierID: supplierID, ChildID: id, Active: active}
	return m, nil
}

func detailCursor(items []SupplierDetailItem, cursor, delta int) int {
	for next := cursor + delta; next >= 0 && next < len(items); next += delta {
		if items[next].Selectable {
			return next
		}
	}
	return cursor
}

func (m SupplierModel) PendingNavigation() (SupplierNavigationTarget, bool) {
	if m.navigation == nil {
		return SupplierNavigationTarget{}, false
	}
	return *m.navigation, true
}
