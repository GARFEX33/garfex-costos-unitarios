package tui

import (
	"context"
	"strings"
)

// FakeAgent keeps fixture data separate from the generic interaction contract.
// It deliberately has no knowledge of the TUI or of any infrastructure.
type FakeAgent struct {
	fixture fixtureContext
}

type fixtureContext struct {
	scenario   string
	values     map[string]string
	pendingKey string
}

func NewFakeAgent() *FakeAgent { return &FakeAgent{} }

func (a *FakeAgent) Respond(_ context.Context, input InteractionInput) (InteractionResponse, error) {
	if input.Kind == InputCancel {
		a.reset()
		return textResponse("Está bien, volvemos a la conversación."), nil
	}
	if input.Kind == InputText {
		a.start(input.Value)
		return a.decide(), nil
	}
	if input.ActionID == "new_search" {
		a.reset()
		return textResponse("Listo. Podés iniciar una nueva búsqueda."), nil
	}
	if input.Kind == InputAction || a.fixture.pendingKey == "confirmation" || a.fixture.pendingKey == "material-intent" || a.fixture.pendingKey == "result-actions" || a.fixture.pendingKey == "error-actions" {
		return a.action(input.ActionID, input.Value), nil
	}
	if input.Key != "" {
		a.fixture.values[input.Key] = input.Value
	} else if a.fixture.pendingKey != "" {
		a.fixture.values[a.fixture.pendingKey] = input.Value
	}
	return a.decide(), nil
}

func (a *FakeAgent) start(input string) {
	value := strings.ToLower(strings.TrimSpace(input))
	a.fixture = fixtureContext{scenario: scenarioFor(value), values: map[string]string{}}
	if a.fixture.scenario == "error" || a.fixture.scenario == "ambiguity" {
		return
	}
	if a.fixture.scenario == "create" {
		return
	}
	if a.fixture.scenario == "cable" {
		a.extractCableValues(value)
	}
}

func scenarioFor(value string) string {
	switch {
	case strings.Contains(value, "error"):
		return "error"
	case strings.Contains(value, "ambigu"):
		return "ambiguity"
	case strings.Contains(value, "crear material"):
		return "create"
	case strings.Contains(value, "tuber") || strings.Contains(value, "tubo"):
		return "pipe"
	case strings.Contains(value, "cable") || strings.Contains(value, "conductor") || strings.Contains(value, "thw-ls") || strings.Contains(value, "xhhw-2"):
		return "cable"
	default:
		return "unknown"
	}
}

func (a *FakeAgent) extractCableValues(value string) {
	for _, insulation := range []string{"thw-ls", "xhhw-2", "bare"} {
		if strings.Contains(value, insulation) {
			a.fixture.values["insulation"] = insulation
		}
	}
	for _, color := range []string{"negro", "blanco", "rojo", "verde"} {
		if strings.Contains(value, color) {
			a.fixture.values["color"] = map[string]string{"negro": "black", "blanco": "white", "rojo": "red", "verde": "green"}[color]
		}
	}
	words := strings.Fields(value)
	for i, word := range words {
		if word == "10" || word == "10awg" || word == "10-awg" {
			a.fixture.values["size"] = "10 AWG"
		} else if i > 0 && strings.Trim(word, "\"'") == "10" {
			a.fixture.values["size"] = "10 AWG"
		}
	}
}

func (a *FakeAgent) decide() InteractionResponse {
	switch a.fixture.scenario {
	case "cable":
		if a.fixture.values["insulation"] == "" {
			return a.question("insulation", "¿Qué aislamiento necesitas?", []Option{{Label: "THW-LS", Value: "thw-ls"}, {Label: "XHHW-2", Value: "xhhw-2"}, {Label: "DESNUDO", Value: "bare"}}, 1, 2)
		}
		if a.fixture.values["color"] == "" {
			return a.question("color", "¿Qué color necesitas?", colorOptions(), 2, 2)
		}
		return materialSummary(a.materialSpec())
	case "pipe":
		if a.fixture.values["type"] == "" {
			return a.question("type", "¿Qué tipo de tubería necesitas?", []Option{{Label: "Conduit pared delgada", Value: "thin"}, {Label: "Conduit pared gruesa", Value: "thick"}, {Label: "PVC conduit", Value: "pvc"}}, 1, 1)
		}
		return pipeResult(a.fixture.values["type"])
	case "create":
		if a.fixture.values["family"] == "" {
			return a.question("family", "¿Qué familia querés crear?", []Option{{Label: "Conductores", Value: "conductors"}, {Label: "Tuberías", Value: "pipes"}}, 1, 2)
		}
		a.fixture.pendingKey = "confirmation"
		return confirmationResponse("¿Crear esta ficha de material?")
	case "ambiguity":
		if a.fixture.values["interpretation"] == "" {
			return a.question("interpretation", "¿Cuál interpretación querés usar?", []Option{{Label: "THW-LS · 10 AWG · Negro", Value: "THW-LS · 10 AWG · Negro"}, {Label: "XHHW-2 · 10 AWG · Negro", Value: "XHHW-2 · 10 AWG · Negro"}}, 1, 1)
		}
		return resultResponseWithActions("MATERIAL ENCONTRADO", "Elegiste una interpretación de la consulta.", []Field{{Label: "Especificación", Value: a.fixture.values["interpretation"]}}, materialActions())
	case "error":
		if a.fixture.values["retry"] == "retry" {
			a.fixture.scenario = "cable"
			a.fixture.values = map[string]string{}
			return a.decide()
		}
		return errorResponse()
	default:
		return textResponse("Contame qué necesitás y, si lo sabés, sus características.")
	}
}

func (a *FakeAgent) question(key, prompt string, options []Option, step, total int) InteractionResponse {
	a.fixture.pendingKey = key
	return questionResponse(QuestionRequest{ID: key, Key: key, Prompt: prompt, Question: prompt, SelectionMode: SelectionSingle, Options: options, Step: step, TotalSteps: total})
}

func (a *FakeAgent) materialSpec() string {
	insulation := map[string]string{"thw-ls": "THW-LS", "xhhw-2": "XHHW-2", "bare": "DESNUDO"}[a.fixture.values["insulation"]]
	if insulation == "" {
		insulation = a.fixture.values["insulation"]
	}
	size := a.fixture.values["size"]
	if size == "" {
		size = "10 AWG"
	}
	color := map[string]string{"black": "Negro", "white": "Blanco", "red": "Rojo", "green": "Verde"}[a.fixture.values["color"]]
	return strings.Trim(strings.Join([]string{insulation, size, color}, " · "), " ·")
}

func pipeResult(value string) InteractionResponse {
	labels := map[string]string{"thin": `1/2" · Conduit pared delgada`, "thick": `1/2" · Conduit pared gruesa`, "pvc": `1/2" · PVC conduit`}
	label := labels[value]
	if label == "" {
		label = value
	}
	return resultResponseWithActions("MATERIAL ENCONTRADO", "Coincidencia simulada para la tubería seleccionada.", []Field{{Label: "Especificación", Value: label}, {Label: "Unidad", Value: "m"}, {Label: "Precio", Value: "simulado"}}, materialActions())
}

func (a *FakeAgent) reset() { a.fixture = fixtureContext{} }

func materialSummary(spec string) InteractionResponse {
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: "Buscaré: " + spec}}, Pending: ActionRequest{ID: "material-intent", Key: "material-intent", Question: "¿Qué querés hacer?", Actions: []Action{{ID: "search", Value: "search", Label: "Buscar", Target: ActionTargetAgent}, {ID: "edit", Value: "edit", Label: "Modificar", Target: ActionTargetAgent}, {ID: "cancel", Value: "cancel", Label: "Cancelar", Target: ActionTargetAgent}}}}
}

func materialActions() []Action {
	return []Action{{ID: "view_prices", Value: "view_prices", Label: "Ver precios", Target: ActionTargetAgent}, {ID: "view_suppliers", Value: "view_suppliers", Label: "Proveedores", Target: ActionTargetAgent}, {ID: "use_in_apu", Value: "use_in_apu", Label: "Usar en APU", Target: ActionTargetAgent}, {ID: "new_search", Value: "new_search", Label: "Nueva búsqueda", Target: ActionTargetLocal}}
}

func (a *FakeAgent) action(id, value string) InteractionResponse {
	if id == "" {
		id = value
	}
	switch id {
	case "new_search":
		a.reset()
		return textResponse("Listo. Podés iniciar una nueva búsqueda.")
	case "search":
		return resultResponseWithActions("MATERIAL ENCONTRADO", "Coincidencia simulada para la ficha seleccionada.", []Field{{Label: "Especificación", Value: a.materialSpec()}, {Label: "Voltaje", Value: "600 V"}, {Label: "Unidad", Value: "m"}, {Label: "Precio", Value: "simulado"}}, materialActions())
	case "view_prices":
		return pricesResponse()
	case "view_suppliers":
		return suppliersResponse()
	case "use_in_apu":
		a.fixture.pendingKey = "confirmation"
		return confirmationResponse("¿Preparar este material para usarlo en APU?")
	case "yes":
		if a.fixture.scenario == "create" {
			a.reset()
			return resultResponseWithActions("MATERIAL PREPARADO", "La ficha puede revisarse antes de guardarla.", []Field{{Label: "Estado", Value: "Listo para crear"}}, nil)
		}
		a.reset()
		return resultResponseWithActions("MATERIAL PREPARADO PARA APU", "La ficha está lista para incorporarse.", []Field{{Label: "Estado", Value: "Preparado"}}, nil)
	case "no", "cancel":
		a.reset()
		return textResponse("La operación fue cancelada.")
	case "back_material":
		return materialSummary(a.materialSpec())
	case "compare_prices":
		return pricesResponse()
	case "retry":
		a.fixture.values["retry"] = "retry"
		return a.decide()
	default:
		return textResponse("Elegí una acción disponible.")
	}
}

func pricesResponse() InteractionResponse {
	return resultResponseWithActions("PRECIOS SIMULADOS", "Valores de fixture para comparar alternativas.", []Field{{Label: "Lista", Value: "Precio de referencia"}, {Label: "Base", Value: "Simulada"}}, []Action{{ID: "view_suppliers", Value: "view_suppliers", Label: "Proveedores", Target: ActionTargetAgent}, {ID: "back_material", Value: "back_material", Label: "Volver al material", Target: ActionTargetLocal}, {ID: "new_search", Value: "new_search", Label: "Nueva búsqueda", Target: ActionTargetLocal}})
}

func suppliersResponse() InteractionResponse {
	return resultResponseWithActions("PROVEEDORES SIMULADOS", "Proveedores de fixture para la ficha seleccionada.", []Field{{Label: "Disponibilidad", Value: "Simulada"}, {Label: "Cobertura", Value: "Local"}}, []Action{{ID: "select_supplier", Value: "select_supplier", Label: "Seleccionar proveedor", Target: ActionTargetAgent}, {ID: "compare_prices", Value: "compare_prices", Label: "Comparar precios", Target: ActionTargetAgent}, {ID: "back_material", Value: "back_material", Label: "Volver al material", Target: ActionTargetLocal}, {ID: "new_search", Value: "new_search", Label: "Nueva búsqueda", Target: ActionTargetLocal}})
}

func confirmationResponse(question string) InteractionResponse {
	return InteractionResponse{Pending: ConfirmationRequest{ID: "confirmation", Key: "confirmation", Question: question, ConfirmLabel: "Confirmar", CancelLabel: "Cancelar"}}
}

func resultResponseWithActions(title, subtitle string, fields []Field, actions []Action) InteractionResponse {
	result := StructuredResult{Title: title, Subtitle: subtitle, Fields: fields}
	response := InteractionResponse{Messages: []InteractionMessage{result}}
	if len(actions) > 0 {
		response.Pending = ActionRequest{ID: "result-actions", Key: "result-actions", Question: "¿Qué querés hacer ahora?", Actions: actions}
	}
	return response
}

func errorResponse() InteractionResponse {
	return InteractionResponse{Messages: []InteractionMessage{ErrorMessage{Text: "No pude completar esa consulta."}}, Pending: ActionRequest{ID: "error-actions", Key: "error-actions", Question: "¿Qué querés hacer?", Actions: []Action{{ID: "retry", Value: "retry", Label: "Reintentar", Target: ActionTargetAgent}, {ID: "new_search", Value: "new_search", Label: "Nueva búsqueda", Target: ActionTargetLocal}, {ID: "cancel", Value: "cancel", Label: "Cancelar", Target: ActionTargetAgent}}}}
}

func textResponse(text string) InteractionResponse {
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: text}}}
}
func questionResponse(question QuestionRequest) InteractionResponse {
	return InteractionResponse{Pending: question}
}
func resultResponse(title string, fields []Field, actions []Action) InteractionResponse {
	return resultResponseWithActions(title, "", fields, actions)
}
func colorOptions() []Option {
	return []Option{{Label: "Negro", Value: "black"}, {Label: "Blanco", Value: "white"}, {Label: "Rojo", Value: "red"}, {Label: "Verde", Value: "green"}}
}
