package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type presentationAgent struct {
	responses []InteractionResponse
	inputs    []InteractionInput
}

func (a *presentationAgent) Respond(_ context.Context, input InteractionInput) (InteractionResponse, error) {
	a.inputs = append(a.inputs, input)
	response := a.responses[0]
	a.responses = a.responses[1:]
	return response, nil
}

func presentationModel(t *testing.T, slug string, agent InteractionAgent) Model {
	t.Helper()
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), []WorkspaceDescriptor{{
		Slug: slug, Title: "GARFEX / " + strings.ToUpper(slug), CreateLabel: "Crear registro", Agent: agent,
	}})
	if !m.enterWorkspace(slug) {
		t.Fatalf("enterWorkspace(%q) = false", slug)
	}
	return m
}

func typeAndSubmit(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, char := range text {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	return m
}

func plainView(m Model) string { return ansi.Strip(m.View().Content) }

func assertViewContains(t *testing.T, m Model, wants ...string) {
	t.Helper()
	view := plainView(m)
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, missing %q", view, want)
		}
	}
}

func assertViewOmits(t *testing.T, m Model, unwanted ...string) {
	t.Helper()
	view := plainView(m)
	for _, value := range unwanted {
		if strings.Contains(view, value) {
			t.Fatalf("view = %q, unexpectedly contains %q", view, value)
		}
	}
}

func TestAssistantPresentationKeepsEarlierTurns(t *testing.T) {
	agent := &presentationAgent{responses: []InteractionResponse{
		{Messages: []InteractionMessage{TextMessage{Text: "Primera respuesta"}}},
		{Messages: []InteractionMessage{TextMessage{Text: "Segunda respuesta"}}},
	}}
	m := NewWithAgent(Handlers{}, agent)
	m = typeAndSubmit(t, m, "primer turno")
	m = typeAndSubmit(t, m, "segundo turno")

	assertViewContains(t, m, "primer turno", "Primera respuesta", "segundo turno", "Segunda respuesta")
}

func TestResourcesPresentationReplacesSearchAndListWithDetail(t *testing.T) {
	agent := &presentationAgent{responses: []InteractionResponse{
		{Pending: QuestionRequest{Key: "results", Prompt: "Resultados cemento", Options: []Option{{Label: "Cemento anterior", Value: "cemento-1"}}}},
		{Messages: []InteractionMessage{StructuredResult{Title: "Detalle vigente", Fields: []Field{{Label: "Código", Value: "cemento-1"}}}}, Pending: ActionRequest{Question: "Acciones vigentes", Actions: []Action{{Label: "Editar", Value: "edit"}}}},
	}}
	m := presentationModel(t, "recursos", agent)
	m, _ = update(t, m, key('b'))
	m = typeAndSubmit(t, m, "cemento")
	assertViewContains(t, m, "Resultados cemento", "Cemento anterior")

	m, _ = update(t, m, enter())
	assertViewContains(t, m, "Detalle vigente", "cemento-1", "Acciones vigentes")
	assertViewOmits(t, m, "Resultados cemento", "Cemento anterior", "❯ cemento")
}

func TestResourcesPresentationReplacesConsecutiveSearches(t *testing.T) {
	agent := &presentationAgent{responses: []InteractionResponse{
		{Messages: []InteractionMessage{TextMessage{Text: "Resultados para cemento"}}},
		{Messages: []InteractionMessage{TextMessage{Text: "Resultados para arena"}}},
	}}
	m := presentationModel(t, "recursos", agent)
	m, _ = update(t, m, key('b'))
	m = typeAndSubmit(t, m, "cemento")
	m, _ = update(t, m, key('b'))
	m = typeAndSubmit(t, m, "arena")

	assertViewContains(t, m, "Resultados para arena")
	assertViewOmits(t, m, "cemento", "Resultados para cemento", "❯ arena")
}

func TestConfigurationPresentationReplacesListDetailAndEditor(t *testing.T) {
	agent := &presentationAgent{responses: []InteractionResponse{
		{Pending: QuestionRequest{Key: "list", Prompt: "Lista anterior", Options: []Option{{Label: "Registro anterior", Value: "1"}}}},
		{Messages: []InteractionMessage{StructuredResult{Title: "Detalle actual"}}, Pending: ActionRequest{Question: "Acciones del detalle", Actions: []Action{{Label: "Editar", Value: "edit"}}}},
		{Messages: []InteractionMessage{TextMessage{Text: "Editor actual"}}, Pending: QuestionRequest{Key: "field", Prompt: "Nombre actual", AllowCustom: true}},
	}}
	m := presentationModel(t, "configuracion", agent)
	m, _ = update(t, m, key('b'))
	m = typeAndSubmit(t, m, "listar")
	m, _ = update(t, m, enter())
	assertViewContains(t, m, "Detalle actual", "Acciones del detalle")
	assertViewOmits(t, m, "Lista anterior", "Registro anterior")

	m, _ = update(t, m, enter())
	assertViewContains(t, m, "Editor actual", "Nombre actual")
	assertViewOmits(t, m, "Detalle actual", "Acciones del detalle", "listar")
}

func TestWorkspaceCancellationReturnsToOriginWithoutTranscript(t *testing.T) {
	agent := &presentationAgent{responses: []InteractionResponse{
		{Messages: []InteractionMessage{TextMessage{Text: "Editor transitorio"}}, Pending: QuestionRequest{Key: "field", Prompt: "Dato transitorio", AllowCustom: true}},
		{Messages: []InteractionMessage{StructuredResult{Title: "Detalle de origen"}}, Pending: ActionRequest{Question: "Acciones de origen", Actions: []Action{{Label: "Volver", Value: "back"}}}},
	}}
	m := presentationModel(t, "recursos", agent)
	m, _ = update(t, m, key('b'))
	m = typeAndSubmit(t, m, "editar")
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))

	assertViewContains(t, m, "Detalle de origen", "Acciones de origen")
	assertViewOmits(t, m, "Editor transitorio", "Dato transitorio", "cancelada", "editar")
	if got := agent.inputs[len(agent.inputs)-1].Kind; got != InputCancel {
		t.Fatalf("last input kind = %v, want InputCancel", got)
	}
}

func TestWorkspaceCompositeResponseKeepsCurrentMessagesAndPending(t *testing.T) {
	agent := &presentationAgent{responses: []InteractionResponse{{
		Messages: []InteractionMessage{TextMessage{Text: "Aviso actual"}, StructuredResult{Title: "Resultado actual"}},
		Pending:  ConfirmationRequest{Question: "Confirmación actual", ConfirmLabel: "Sí", CancelLabel: "No"},
	}}}
	m := presentationModel(t, "recursos", agent)
	m, _ = update(t, m, key('b'))
	m = typeAndSubmit(t, m, "consulta vieja")

	assertViewContains(t, m, "Aviso actual", "Resultado actual", "Confirmación actual")
	assertViewOmits(t, m, "consulta vieja")
}

func TestWorkspaceMenuNavigationDoesNotRequireSlashAndSlashRemainsOptional(t *testing.T) {
	agent := &presentationAgent{}
	m := presentationModel(t, "recursos", agent)
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = update(t, m, enter())
	if m.interactionMode != interactionModeChat || !m.inputFocused {
		t.Fatalf("arrow/Enter navigation mode=%v focused=%v, want focused chat", m.interactionMode, m.inputFocused)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m, _ = update(t, m, key('/'))
	if m.interactionMode != interactionModePalette {
		t.Fatalf("slash mode = %v, want palette", m.interactionMode)
	}
}
