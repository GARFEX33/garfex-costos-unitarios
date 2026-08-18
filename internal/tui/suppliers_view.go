package tui

import (
	"strings"
)

func (m SupplierModel) supplierView() string {
	switch frame := m.frame.(type) {
	case SupplierManagerFrame:
		return strings.Join([]string{"Proveedores", frame.StateText(), supplierFooter("manager")}, "\n")
	case SupplierDetailFrame:
		return strings.Join([]string{"Detalle del proveedor", frame.StateText(), supplierFooter("detail")}, "\n")
	case SupplierEditFrame:
		return supplierEditView(frame)
	default:
		return ""
	}
}

func supplierEditView(frame SupplierEditFrame) string {
	title := "Editar proveedor"
	if !frame.Mode {
		title = "Crear proveedor"
	}
	lines := []string{title}
	values := []string{frame.Values.TradeName, frame.Values.LegalName, frame.Values.TaxIdentifier, frame.Values.Website, frame.Values.Notes}
	for i, label := range supplierFormFields {
		marker := " "
		if i == frame.Focus {
			marker = ">"
		}
		lines = append(lines, marker+label+": "+values[i])
	}
	if frame.Error != "" {
		lines = append(lines, frame.Error)
	}
	lines = append(lines, supplierFooter("edit"))
	return strings.Join(lines, "\n")
}

var supplierFooters = map[string]string{
	"manager": "Ctrl+N crear · Enter detalle · Ctrl+C salir",
	"detail":  "E editar · Esc volver · Ctrl+C salir",
	"edit":    "Esc: Cancelar · Ctrl+C salir",
}

func supplierFooter(route string) string { return supplierFooters[route] }
