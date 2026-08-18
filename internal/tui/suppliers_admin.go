package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
	"reflect"
)

type BranchEditFrame struct {
	RouteID, RequestID   uint64
	SupplierID, BranchID int64
	Mode                 bool
	Values               domain.BranchDetails
	Focus                int
	Focused              bool
}
type ContactEditFrame struct {
	RouteID, RequestID    uint64
	SupplierID, ContactID int64
	Mode                  bool
	Values                domain.ContactDetails
	Focus                 int
	Focused               bool
}

var branchFormFields = []string{
	"Nombre", "Referencia", "Ciudad", "Provincia/Estado", "País",
	"Dirección", "Teléfono general", "Email general", "Notas",
}
var contactFormFields = []string{"Nombre", "Cargo", "Sucursal", "Móvil", "Teléfono", "Email", "Notas"}

func branchDetails(value domain.Branch) domain.BranchDetails {
	return domain.BranchDetails{Name: value.Name, Reference: value.Reference, City: value.City, State: value.State, Country: value.Country, Address: value.Address, GeneralPhone: value.GeneralPhone, GeneralEmail: value.GeneralEmail, Notes: value.Notes}
}
func contactDetails(value domain.Contact) domain.ContactDetails {
	var branchID *int64
	if value.BranchID != nil {
		copyID := *value.BranchID
		branchID = &copyID
	}
	return domain.ContactDetails{BranchID: branchID, Name: value.Name, Role: value.Role, Phone: value.Phone, Mobile: value.Mobile, Email: value.Email, Notes: value.Notes}
}

var branchTextFields = []string{"Name", "Reference", "City", "State", "Country", "Address", "GeneralPhone", "GeneralEmail", "Notes"}
var contactTextFields = []string{"Name", "Role", "", "Mobile", "Phone", "Email", "Notes"}

func appendText(value any, focus int, text string, fields []string) any {
	if focus >= len(fields) || fields[focus] == "" {
		return value
	}
	result := reflect.New(reflect.TypeOf(value)).Elem()
	result.Set(reflect.ValueOf(value))
	field := result.FieldByName(fields[focus])
	field.SetString(field.String() + text)
	return result.Interface()
}
func appendBranchText(frame BranchEditFrame, text string) BranchEditFrame {
	frame.Values = appendText(frame.Values, frame.Focus, text, branchTextFields).(domain.BranchDetails)
	return frame
}
func appendContactText(frame ContactEditFrame, text string) ContactEditFrame {
	frame.Values = appendText(frame.Values, frame.Focus, text, contactTextFields).(domain.ContactDetails)
	return frame
}
func boundedFormFocus(focus, fieldCount int) int {
	if focus < 0 {
		return 0
	}
	if focus >= fieldCount {
		return fieldCount - 1
	}
	return focus
}
func editKey(msg tea.KeyPressMsg, focus *int, fields int, focused bool, appendText func(string)) bool {
	if focused {
		if text := supplierPrintableText(msg); text != "" {
			appendText(text)
			return true
		}
	}
	if delta, ok := supplierKeyDelta[msg.String()]; ok {
		*focus = boundedFormFocus(*focus+delta, fields)
		return true
	}
	if msg.String() == "tab" {
		*focus = (*focus + 1) % fields
		return true
	}
	return msg.String() == "esc"
}
func (m SupplierModel) updateChildEdit(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch frame := m.frame.(type) {
	case BranchEditFrame:
		if !editKey(msg, &frame.Focus, len(branchFormFields), frame.Focused, func(text string) { frame = appendBranchText(frame, text) }) {
			return m, nil, false
		}
		if msg.String() == "esc" {
			next, cmd := m.popFrame()
			return next, cmd, true
		}
		m.frame = frame
		return m, nil, true
	case ContactEditFrame:
		if !editKey(msg, &frame.Focus, len(contactFormFields), frame.Focused, func(text string) { frame = appendContactText(frame, text) }) {
			return m, nil, false
		}
		if msg.String() == "esc" {
			next, cmd := m.popFrame()
			return next, cmd, true
		}
		m.frame = frame
		return m, nil, true
	default:
		return m, nil, false
	}
}
func (m SupplierModel) openBranchEdit(previous any, value domain.Branch, mode bool) (SupplierModel, tea.Cmd) {
	if frame, ok := previous.(BranchDetailFrame); ok {
		if value.SupplierID == 0 {
			value.SupplierID = frame.SupplierID
		}
		if value.ID == 0 {
			value.ID = frame.BranchID
		}
	}
	m.pushFrame(previous)
	m.frame = BranchEditFrame{
		RouteID: m.nextRouteID, RequestID: m.nextRequestID, SupplierID: value.SupplierID,
		BranchID: value.ID, Mode: mode, Values: branchDetails(value), Focused: true,
	}
	return m, nil
}
func (m SupplierModel) openContactEdit(previous any, value domain.Contact, mode bool) (SupplierModel, tea.Cmd) {
	switch frame := previous.(type) {
	case ContactDetailFrame:
		if value.SupplierID == 0 {
			value.SupplierID = frame.SupplierID
		}
		if value.ID == 0 {
			value.ID = frame.ContactID
		}
		if value.BranchID == nil && frame.BranchID != nil {
			branchID := *frame.BranchID
			value.BranchID = &branchID
		}
	case ContactManagerFrame:
		if value.SupplierID == 0 {
			value.SupplierID = frame.SupplierID
		}
	}
	m.pushFrame(previous)
	m.frame = ContactEditFrame{
		RouteID: m.nextRouteID, RequestID: m.nextRequestID, SupplierID: value.SupplierID,
		ContactID: value.ID, Mode: mode, Values: contactDetails(value), Focused: true,
	}
	return m, nil
}
