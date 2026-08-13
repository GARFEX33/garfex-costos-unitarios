package tui

import "context"

// AssistantShellAgent is the production InteractionAgent for the Assistant's
// main chat (GARFEX / ASSISTANT). It is an honest placeholder: there is no
// LLM or orchestrator yet, so it must never pretend to interpret free text.
// Specialized capabilities (starting with Materiales Maestros) are reached
// explicitly via the "/" command palette, which opens their own independent
// workspace (see enterMaterialsWorkspace in model.go) — this agent never
// redirects to one based on input content.
type AssistantShellAgent struct{}

// NewAssistantShellAgent returns the Assistant's default production agent.
// It is a pure, dependency-free stub: it never touches Postgres,
// materialSearcher, or any application service.
func NewAssistantShellAgent() *AssistantShellAgent { return &AssistantShellAgent{} }

// assistantShellPlaceholder is honest about the current lack of a real
// conversational capability and points the user at "/" as the way to reach
// one, instead of claiming to understand free text.
const assistantShellPlaceholder = "Todavía no interpreto lenguaje natural. Escribí / para elegir una capacidad, por ejemplo Materiales Maestros."

// Greeting is shown once, before any user input, so the Assistant's honest
// scope is visible from the start of the workspace instead of only
// appearing after the user types something — mirrors the pattern already
// established by MaterialsWorkspaceAdapter.Greeting().
func (a *AssistantShellAgent) Greeting() InteractionMessage {
	return TextMessage{Text: assistantShellPlaceholder}
}

// Respond never simulates understanding: every InteractionInput, regardless
// of Kind or Value, receives the same honest placeholder pointing at "/".
func (a *AssistantShellAgent) Respond(context.Context, InteractionInput) (InteractionResponse, error) {
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: assistantShellPlaceholder}}}, nil
}
