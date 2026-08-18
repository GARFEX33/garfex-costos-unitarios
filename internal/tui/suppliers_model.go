package tui

import tea "charm.land/bubbletea/v2"

type SupplierModel struct {
	service       SupplierListService
	frame         SupplierManagerFrame
	nextRequestID uint64
	AtRoot        bool
}

var supplierKeyDelta = map[string]int{"up": -1, "k": -1, "down": 1, "j": 1}

func NewSupplierModel(service SupplierListService) SupplierModel {
	return SupplierModel{
		service:       service,
		frame:         SupplierManagerFrame{RouteID: 1, RequestID: 1, Filter: SupplierFilterActive, State: SupplierStateLoading, Viewport: 10},
		nextRequestID: 1,
	}
}

func (m SupplierModel) Init() tea.Cmd { return supplierListCmd(m.service, m.frame) }

func (m SupplierModel) CurrentFrame() any { return m.frame }

func (m SupplierModel) View() tea.View { return tea.NewView("Proveedores") }

func (m SupplierModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case SupplierListMsg:
		return m.listReply(value), nil
	case tea.KeyPressMsg:
		return m.updateKey(value)
	default:
		return m, nil
	}
}

func (m SupplierModel) listReply(msg SupplierListMsg) SupplierModel {
	if m.frame.RouteID != msg.RouteID || m.frame.RequestID != msg.RequestID {
		return m
	}
	m.frame.Rows = msg.Rows
	m.frame.State, m.frame.Error = supplierResultState(msg.Err, len(msg.Rows) == 0)
	if m.frame.SelectedID == 0 && len(msg.Rows) > 0 {
		m.frame.SelectedID = msg.Rows[0].ID
		m.frame.Cursor = 0
	}
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
	m.frame.Query, m.frame.Filter = query, filter
	m.frame.Rows, m.frame.SelectedID, m.frame.Cursor, m.frame.Offset = nil, 0, 0, 0
	m.nextRequestID++
	m.frame.RequestID = m.nextRequestID
	m.frame.State, m.frame.Error = SupplierStateLoading, ""
	return m, supplierListCmd(m.service, m.frame)
}

func (m SupplierModel) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if delta, ok := supplierKeyDelta[msg.String()]; ok {
		next := m.frame.Cursor + delta
		if next >= 0 && next < len(m.frame.Rows) {
			m.frame.Cursor = next
			m.frame.SelectedID = m.frame.Rows[next].ID
		}
		return m, nil
	}
	if msg.String() == "esc" {
		m.AtRoot = true
	}
	return m, nil
}
