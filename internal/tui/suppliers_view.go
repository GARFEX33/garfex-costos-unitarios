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
	case SupplierLifecycleFrame:
		return supplierLifecycleView(frame)
	case SupplierHelpFrame:
		return supplierHelpView(frame)
	case BranchManagerFrame:
		return m.branchManagerView(frame)
	case ContactManagerFrame:
		return m.contactManagerView(frame)
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

func supplierLifecycleView(frame SupplierLifecycleFrame) string {
	state := "Inactivo"
	action := "Reactivar"
	if frame.Active {
		state = "Activo"
		action = "Desactivar"
	}
	lines := []string{"Cambiar estado del proveedor", "Estado actual: " + state, "Acción: " + action}
	if frame.Error != "" {
		lines = append(lines, frame.Error)
	}
	lines = append(lines, supplierFooter("confirm"))
	return strings.Join(lines, "\n")
}

func supplierHelpView(frame SupplierHelpFrame) string {
	lines := []string{"Ayuda de Proveedores", "Las teclas escriben texto cuando un campo está enfocado.", "Ctrl+C sale de GARFEX."}
	switch frame.Origin {
	case "manager":
		lines = append(lines, "Ctrl+N crear · Enter detalle · ? ayuda")
	case "detail":
		lines = append(lines, "E editar · A estado · Esc volver · ? ayuda")
	case "edit":
		lines = append(lines, "Enter guardar · Esc: Cancelar")
	default:
		lines = append(lines, "Enter confirmar · Cancelar · Esc volver")
	}
	lines = append(lines, supplierFooter("help"))
	return strings.Join(lines, "\n")
}

var supplierFooters = map[string]string{
	"manager": "Ctrl+N crear · Enter detalle · ? ayuda · Esc volver · Ctrl+C salir",
	"detail":  "E editar · A estado · ? ayuda · Esc volver · Ctrl+C salir",
	"edit":    "Enter guardar · Esc: Cancelar · Ctrl+C salir",
	"confirm": "Enter confirmar · Cancelar · Esc volver · Ctrl+C salir",
	"help":    "Esc volver · Ctrl+C salir",
}

func supplierFooter(route string) string { return supplierFooters[route] }
