package tui

import (
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

func supplierResultState(err error, empty bool) (SupplierState, string) {
	if err != nil {
		return SupplierStateError, "No pude cargar los proveedores. Probá de nuevo en un momento."
	}
	if empty {
		return SupplierStateEmpty, ""
	}
	return SupplierStateReady, ""
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
	case SupplierDetailFrame:
		if msg.String() == "e" || msg.String() == "E" {
			supplier := frame.Supplier
			if supplier.ID == 0 {
				supplier.ID = frame.SupplierID
			}
			return m.openEdit(frame, supplier, SupplierEditUpdate)
		}
		if msg.String() == "enter" && frame.Cursor >= 0 && frame.Cursor < len(frame.Items) && frame.Items[frame.Cursor].Selectable {
			target := frame.Items[frame.Cursor].Target
			m.navigation = &target
		}
		if msg.String() == "esc" && len(m.stack) > 0 {
			m.frame = m.stack[len(m.stack)-1]
			m.stack = m.stack[:len(m.stack)-1]
			m.navigation = nil
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
	}
	return m, nil
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
