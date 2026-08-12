package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func multiSelectModel(agent InteractionAgent, request QuestionRequest) Model {
	m := NewWithAgent(Handlers{}, agent)
	m.screen = screenWorkspace
	m.pending = request
	m.interactionMode = interactionModeChoice
	m.syncChoiceFields()
	m.refreshViewport()
	return m
}

func multiSelectKey(t *testing.T, m Model, key string) Model {
	t.Helper()
	message := tea.KeyPressMsg(tea.Key{Text: key})
	if key == "enter" {
		message = tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	}
	if key == " " {
		message = tea.KeyPressMsg(tea.Key{Code: tea.KeySpace})
	}
	next, _ := m.Update(message)
	return next.(Model)
}

func multiSelectRequest(minimum, maximum int) QuestionRequest {
	return QuestionRequest{
		Key:           "providers",
		Prompt:        "Choose providers",
		SelectionMode: SelectionMultiple,
		MinSelections: minimum,
		MaxSelections: maximum,
		Options: []Option{
			{Label: "North", Value: "north"},
			{Label: "Center", Value: "center"},
			{Label: "South", Value: "south"},
		},
	}
}

func TestMultiSelectStateTransitions(t *testing.T) {
	m := multiSelectModel(&multiSelectAgent{}, multiSelectRequest(0, 0))
	for _, key := range []string{" ", "down", " ", "up", " "} {
		m = multiSelectKey(t, m, key)
	}
	if got := m.choiceSelected; len(got) != 3 || got[0] || !got[1] || got[2] {
		t.Fatalf("selected state = %#v, want [false true false]", got)
	}
	m = multiSelectKey(t, m, "j")
	if m.choiceIndex != 1 {
		t.Fatalf("j moved cursor to %d, want 1", m.choiceIndex)
	}
	m = multiSelectKey(t, m, "k")
	if m.choiceIndex != 0 {
		t.Fatalf("k moved cursor to %d, want 0", m.choiceIndex)
	}
}

func TestMultiSelectValidationAndMaxEnforcement(t *testing.T) {
	agent := &multiSelectAgent{}
	m := multiSelectModel(agent, multiSelectRequest(2, 2))
	m = multiSelectKey(t, m, "enter")
	if agent.calls != 0 || m.pending == nil {
		t.Fatalf("confirmed below minimum: calls=%d pending=%T", agent.calls, m.pending)
	}
	if !strings.Contains(ansi.Strip(m.renderActiveInteraction(m.pending, m.choiceIndex)), "Faltan selecciones") {
		t.Fatal("minimum-unmet status is missing")
	}
	m = multiSelectKey(t, m, " ")
	m = multiSelectKey(t, m, "down")
	m = multiSelectKey(t, m, " ")
	m = multiSelectKey(t, m, "down")
	m = multiSelectKey(t, m, " ")
	if m.choiceSelected[2] {
		t.Fatalf("selected beyond maximum: %#v", m.choiceSelected)
	}
	if !strings.Contains(ansi.Strip(m.renderActiveInteraction(m.pending, m.choiceIndex)), "Máximo alcanzado") {
		t.Fatal("maximum-reached status is missing")
	}
	m = multiSelectKey(t, m, "up")
	m = multiSelectKey(t, m, " ")
	m = multiSelectKey(t, m, "down")
	m = multiSelectKey(t, m, " ")
	if !m.choiceSelected[2] || m.selectedCount() != 2 {
		t.Fatalf("deselection did not free max slot: %#v", m.choiceSelected)
	}
}

func TestMultiSelectConfirmsEmptySelectionWhenValid(t *testing.T) {
	agent := &multiSelectAgent{}
	request := multiSelectRequest(0, 0)
	request.Options = nil
	m := multiSelectModel(agent, request)
	m = multiSelectKey(t, m, "enter")
	if agent.calls != 1 || len(agent.inputs) != 1 || len(agent.inputs[0].Values) != 0 {
		t.Fatalf("empty selection = calls %d, inputs %#v", agent.calls, agent.inputs)
	}
	if m.pending != nil || m.interactionMode != interactionModeChat {
		t.Fatalf("valid empty selection remained pending: pending=%T mode=%v", m.pending, m.interactionMode)
	}
}

func TestMultiSelectDoesNotConfirmEmptySelectionWhenInvalid(t *testing.T) {
	agent := &multiSelectAgent{}
	request := multiSelectRequest(1, 0)
	request.Options = nil
	m := multiSelectModel(agent, request)
	m = multiSelectKey(t, m, "enter")
	if agent.calls != 0 || m.pending == nil {
		t.Fatalf("invalid empty selection confirmed: calls=%d pending=%T", agent.calls, m.pending)
	}
}

func TestMultiSelectConstraintEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		minimum int
		maximum int
		want    bool
	}{
		{name: "negative minimum is no minimum", minimum: -1, maximum: 1, want: true},
		{name: "zero maximum is unlimited", minimum: 2, maximum: 0, want: false},
		{name: "contradictory bounds are invalid", minimum: 2, maximum: 1, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := multiSelectRequest(tt.minimum, tt.maximum)
			m := multiSelectModel(&multiSelectAgent{}, request)
			if got := m.multipleSelectionValid(request); got != tt.want {
				t.Fatalf("valid = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInteractionDockShowsToggleHintOnlyForMultiSelect(t *testing.T) {
	tests := []struct {
		name     string
		pending  InteractionMessage
		contains bool
	}{
		{name: "multiple question", pending: multiSelectRequest(0, 0), contains: true},
		{name: "single question", pending: QuestionRequest{SelectionMode: SelectionSingle, Options: []Option{{Label: "One"}}}},
		{name: "confirmation", pending: ConfirmationRequest{Question: "Continue?"}},
		{name: "action", pending: ActionRequest{Question: "Choose", Actions: []Action{{Label: "Run"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{screen: screenWorkspace, pending: tt.pending, interactionMode: modeFor(tt.pending)}
			hint := ansi.Strip(m.renderInteractionDock(80))
			if got := strings.Contains(hint, "Espacio alternar"); got != tt.contains {
				t.Fatalf("toggle hint = %v in %q, want %v", got, hint, tt.contains)
			}
			footer := ansi.Strip(m.renderFooter(80))
			if got := strings.Contains(footer, "alternar"); got != tt.contains {
				t.Fatalf("footer toggle hint = %v in %q, want %v", got, footer, tt.contains)
			}
		})
	}
}

func TestMultiSelectReturnsOptionOrderAndPreservesCancellation(t *testing.T) {
	agent := &multiSelectAgent{}
	m := multiSelectModel(agent, multiSelectRequest(1, 0))
	m = multiSelectKey(t, m, "down")
	m = multiSelectKey(t, m, "down")
	m = multiSelectKey(t, m, " ")
	m = multiSelectKey(t, m, "up")
	m = multiSelectKey(t, m, "up")
	m = multiSelectKey(t, m, " ")
	m = multiSelectKey(t, m, "enter")
	if len(agent.inputs) != 1 || len(agent.inputs[0].Values) != 2 || agent.inputs[0].Values[0] != "north" || agent.inputs[0].Values[1] != "south" {
		t.Fatalf("values = %#v, want option order [north south]", agent.inputs)
	}
	resolved := ""
	for _, message := range m.history {
		if message.resolved != nil {
			resolved = message.resolved.selection
		}
	}
	if resolved != "North, South" {
		t.Fatalf("resolved labels = %q", resolved)
	}

	m = multiSelectModel(agent, multiSelectRequest(1, 0))
	m = multiSelectKey(t, m, "esc")
	if agent.inputs[len(agent.inputs)-1].Kind != InputCancel || m.pending != nil || m.interactionMode != interactionModeChat {
		t.Fatalf("cancel = input %#v pending=%T mode=%v", agent.inputs[len(agent.inputs)-1], m.pending, m.interactionMode)
	}
}

func TestMultiSelectRenderingDistinguishesFocusSelectionAndStatus(t *testing.T) {
	m := multiSelectModel(&multiSelectAgent{}, multiSelectRequest(1, 2))
	m = multiSelectKey(t, m, " ")
	plain := ansi.Strip(m.renderActiveInteraction(m.pending, m.choiceIndex))
	for _, want := range []string{"Seleccionadas: 1", "❯ [x] North", "  [ ] Center", "  [ ] South"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("render missing %q: %q", want, plain)
		}
	}
	m = multiSelectKey(t, m, "down")
	plain = ansi.Strip(m.renderActiveInteraction(m.pending, m.choiceIndex))
	if !strings.Contains(plain, "  [x] North") || !strings.Contains(plain, "❯ [ ] Center") {
		t.Fatalf("focus/selection changed incorrectly: %q", plain)
	}
}

func TestMultiSelectSelectionStatus(t *testing.T) {
	tests := []struct {
		name    string
		request QuestionRequest
		count   int
		want    []string
	}{
		{name: "minimum unmet", request: multiSelectRequest(2, 3), count: 1, want: []string{"Seleccionadas: 1", "Mínimo: 2", "Máximo: 3", "Faltan selecciones"}},
		{name: "maximum reached", request: multiSelectRequest(1, 2), count: 2, want: []string{"Seleccionadas: 2", "Máximo alcanzado"}},
		{name: "unlimited", request: multiSelectRequest(0, 0), count: 3, want: []string{"Seleccionadas: 3", "Sin máximo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := selectionStatus(tt.request, tt.count)
			for _, want := range tt.want {
				if !strings.Contains(status, want) {
					t.Fatalf("status = %q, missing %q", status, want)
				}
			}
		})
	}
}

type multiSelectAgent struct {
	inputs []InteractionInput
	calls  int
}

func (a *multiSelectAgent) Respond(_ context.Context, input InteractionInput) (InteractionResponse, error) {
	a.inputs = append(a.inputs, input)
	a.calls++
	return textResponse("received"), nil
}
