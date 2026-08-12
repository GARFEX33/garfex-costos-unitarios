package tui

import "context"

// MaterialsWorkspaceAdapter is the production InteractionAgent for the
// Materiales Maestros workspace. It is a thin TUI-to-application-service
// adapter — not the Materials domain, not materiales.Service itself, and
// not the future LLM-driven agent. Search is not implemented yet; it
// deliberately never simulates a question or a fabricated result, only an
// honest status message.
type MaterialsWorkspaceAdapter struct {
	materials materialGetter
}

// NewMaterialsWorkspaceAdapter returns the production agent for the
// Materiales Maestros workspace. getter is satisfied structurally by
// *materiales.Service (see internal/tui/handlers.go's materialGetter,
// reused here rather than duplicated) — composed for real in
// cmd/garfex/main.go. It is not called yet by Respond/Greeting in this
// change; Search (a later PR) will use it.
func NewMaterialsWorkspaceAdapter(getter materialGetter) *MaterialsWorkspaceAdapter {
	return &MaterialsWorkspaceAdapter{materials: getter}
}

const materialsStatusMessage = "Materiales Maestros está conectado al catálogo real (PostgreSQL). La búsqueda todavía no está implementada — llega en un próximo cambio."

// Greeting is shown once, before any user input, so the connection status
// is visible from the start of the workspace instead of only appearing
// after the user types something.
func (a *MaterialsWorkspaceAdapter) Greeting() InteractionMessage {
	return TextMessage{Text: materialsStatusMessage}
}

// Respond never simulates a question or a result — it always returns the
// same honest status message, regardless of input, until Search exists.
func (a *MaterialsWorkspaceAdapter) Respond(context.Context, InteractionInput) (InteractionResponse, error) {
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: materialsStatusMessage}}}, nil
}
