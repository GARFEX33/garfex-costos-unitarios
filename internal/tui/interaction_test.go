package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestInteractionContracts(t *testing.T) {
	var _ InteractionAgent = NewFakeAgent()
	response := InteractionResponse{Messages: []InteractionMessage{
		TextMessage{Text: "text"},
		QuestionRequest{ID: "q", Key: "choice", Question: "Pick", Options: []Option{{Label: "One", Value: "one"}}},
		ConfirmationRequest{ID: "c", Key: "confirm", Question: "Continue?", ConfirmLabel: "Confirmar", CancelLabel: "Cancelar"},
		ActionRequest{ID: "a", Key: "action", Question: "Choose", Actions: []Action{{ID: "run", Value: "run", Label: "Run"}}},
		StructuredResult{Title: "Result", Subtitle: "Details", Fields: []Field{{Label: "State", Value: "ready"}}, Sections: []Section{{Title: "Extra", Fields: []Field{{Label: "Code", Value: "A1"}}}}, Actions: []Action{{ID: "use", Value: "use", Label: "Use"}}},
		ErrorMessage{Text: "failed"},
	}}
	for _, message := range response.Messages {
		if message == nil {
			t.Fatal("nil semantic message")
		}
	}
}

func TestSelectionContractKeepsSingleValueAndAllowsFutureValues(t *testing.T) {
	request := QuestionRequest{
		SelectionMode: SelectionMultiple,
		MinSelections: 1,
		MaxSelections: 3,
	}
	if request.MinSelections != 1 || request.MaxSelections != 3 {
		t.Fatalf("selection constraints = %d/%d", request.MinSelections, request.MaxSelections)
	}

	single := InteractionInput{Kind: InputSelection, Key: "insulation", Value: "thw-ls"}
	if single.Value != "thw-ls" || len(single.Values) != 0 {
		t.Fatalf("single selection compatibility = %#v", single)
	}

	multiple := InteractionInput{Kind: InputSelection, Key: "providers", Values: []string{"north", "center"}}
	if len(multiple.Values) != 2 || multiple.Value != "" {
		t.Fatalf("multiple selection contract = %#v", multiple)
	}
}

func TestInteractionResponseAcceptsUnknownSemanticMessage(t *testing.T) {
	response := InteractionResponse{
		Messages: []InteractionMessage{externalSemanticMessage{Text: "external"}},
		Pending:  externalSemanticMessage{Text: "pending"},
	}

	if len(response.Messages) != 1 || response.Pending == nil {
		t.Fatalf("response did not retain external semantic message: %#v", response)
	}
	if got := renderInteractionMessage(response.Messages[0]); got != "" {
		t.Fatalf("unknown message rendered as %q, want empty output", got)
	}
}

type externalSemanticMessage struct {
	Text string
}

func (externalSemanticMessage) InteractionMessage() {}

func TestQuestionPromptIsTheOnlyVisibleQuestionText(t *testing.T) {
	request := QuestionRequest{Prompt: "Visible prompt", Question: "internal fallback"}
	if got := renderInteractionMessage(request); got != "Visible prompt" {
		t.Fatalf("rendered prompt = %q", got)
	}
	if got := renderInteractionMessage(QuestionRequest{Question: "compatible prompt"}); got != "compatible prompt" {
		t.Fatalf("question alias = %q", got)
	}
}

func TestFakeAgentUsesKnownAttributesAndReturnsGenericAction(t *testing.T) {
	engine := NewInteractionEngine(NewFakeAgent())
	response := engine.Text(context.Background(), "THW-LS 10 negro")
	if _, ok := response.Pending.(ActionRequest); !ok {
		t.Fatalf("complete fixture pending = %T, want action", response.Pending)
	}
	if _, ok := firstMessage(response).(QuestionRequest); ok {
		t.Fatal("complete fixture unexpectedly asked a question")
	}
	if !hasText(response, "Buscaré:") {
		t.Fatalf("complete fixture messages = %#v", response.Messages)
	}
}

func TestScenarioForRecognizesNaturalMaterialRequestWithoutTechnicalTrigger(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "quiero buscar un material", want: "multi"},
		{input: "buscar material por características", want: "multi"},
		{input: "multi", want: "unknown"},
		{input: "multiselect", want: "unknown"},
		{input: "cable", want: "cable"},
		{input: "tubería", want: "pipe"},
		{input: "crear material", want: "create"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := scenarioFor(tt.input); got != tt.want {
				t.Fatalf("scenarioFor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFakeAgentMultiSelectResponseShape(t *testing.T) {
	agent := NewFakeAgent()
	got, err := agent.Respond(context.Background(), InteractionInput{Kind: InputText, Value: "quiero buscar un material"})
	if err != nil {
		t.Fatalf("start multi-select: %v", err)
	}
	question, ok := got.Pending.(QuestionRequest)
	if !ok {
		t.Fatalf("pending = %T, want question", got.Pending)
	}
	if question.SelectionMode != SelectionMultiple || question.MinSelections != 2 || question.MaxSelections != 3 || question.AllowCustom {
		t.Fatalf("question constraints = %#v", question)
	}
	if len(question.Options) != 5 || question.Key != "materials" || question.Prompt != "Selecciona las características que quieres considerar" {
		t.Fatalf("question options/key = %d/%q", len(question.Options), question.Key)
	}
}

func TestFakeAgentHandlesMultiSelectValuesPayload(t *testing.T) {
	engine := NewInteractionEngine(NewFakeAgent())
	engine.Text(context.Background(), "quiero buscar un material")
	response := engine.Respond(context.Background(), InteractionInput{
		Kind:   InputSelection,
		Key:    "materials",
		Values: []string{"thw-ls", "cobre"},
	})
	result, ok := firstStructured(response)
	if !ok || response.Pending != nil {
		t.Fatalf("response = %#v pending=%T, want result without pending question", response.Messages, response.Pending)
	}
	if result.Title != "SELECCIÓN MÚLTIPLE RECIBIDA" || len(result.Fields) != 1 || result.Fields[0].Value != "thw-ls, cobre" {
		t.Fatalf("multi-select result = %#v", result)
	}
}

func TestInteractionEngineClearsPendingWhenResponseIsNotPending(t *testing.T) {
	engine := NewInteractionEngine(NewFakeAgent())
	first := engine.Text(context.Background(), "cable")
	if _, ok := first.Pending.(QuestionRequest); !ok {
		t.Fatalf("first pending = %T", first.Pending)
	}
	second := engine.Cancel(context.Background())
	if second.Pending != nil || engine.Pending() != nil {
		t.Fatalf("pending survived cancellation: response=%T engine=%T", second.Pending, engine.Pending())
	}
	if len(engine.History()) != len(first.Messages)+len(second.Messages) {
		t.Fatalf("history length = %d", len(engine.History()))
	}
}

func TestFakeAgentGuidedCableChain(t *testing.T) {
	engine := NewInteractionEngine(NewFakeAgent())
	assertPendingType(t, engine.Text(context.Background(), "cable del 10"), QuestionRequest{})
	assertPendingType(t, engine.Select(context.Background(), "thw-ls"), QuestionRequest{})
	response := engine.Select(context.Background(), "black")
	actions := pendingActions(t, response)
	assertActionValues(t, actions, "search", "edit", "cancel")
	result := engine.Action(context.Background(), "search")
	assertStructuredWithPendingActions(t, result, "view_prices", "view_suppliers", "use_in_apu", "new_search")
}

func TestStructuredResultDoesNotDuplicatePendingActions(t *testing.T) {
	response := resultResponseWithActions("PRECIOS SIMULADOS", "Valores de fixture.", nil, []Action{{ID: "view_prices", Value: "view_prices", Label: "Ver precios"}})
	result, ok := firstStructured(response)
	if !ok {
		t.Fatal("response does not contain a structured result")
	}
	if len(result.Actions) != 0 {
		t.Fatalf("structured result actions = %#v, want empty", result.Actions)
	}
	request, ok := response.Pending.(ActionRequest)
	if !ok || len(request.Actions) != 1 || request.Actions[0].ID != "view_prices" || request.Actions[0].Value != "view_prices" {
		t.Fatalf("pending = %#v, want action ID and value", response.Pending)
	}
	if rendered := renderInteractionMessage(result); containsFold(rendered, "Ver precios") || strings.Contains(rendered, "[") {
		t.Fatalf("structured result rendered decorative action: %q", rendered)
	}
}

func TestFakeAgentPricesProvidersAndAPU(t *testing.T) {
	engine := NewInteractionEngine(NewFakeAgent())
	engine.Text(context.Background(), "cable")
	engine.Select(context.Background(), "thw-ls")
	engine.Select(context.Background(), "black")
	prices := engine.Action(context.Background(), "search")
	prices = engine.Action(context.Background(), "view_prices")
	assertStructuredWithPendingActions(t, prices, "view_suppliers", "back_material", "new_search")
	providers := engine.Action(context.Background(), "view_suppliers")
	assertStructuredWithPendingActions(t, providers, "select_supplier", "compare_prices", "back_material", "new_search")
	apu := engine.Action(context.Background(), "use_in_apu")
	assertPendingType(t, apu, ConfirmationRequest{})
	prepared := engine.Select(context.Background(), "yes")
	if prepared.Pending != nil || !hasStructuredTitle(prepared, "MATERIAL PREPARADO PARA APU") {
		t.Fatalf("APU result = %#v pending=%T", prepared.Messages, prepared.Pending)
	}
}

func TestFakeAgentTubeReusesPostSearchActions(t *testing.T) {
	engine := NewInteractionEngine(NewFakeAgent())
	assertPendingType(t, engine.Text(context.Background(), "tubería de media"), QuestionRequest{})
	result := engine.Select(context.Background(), "thick")
	assertStructuredWithPendingActions(t, result, "view_prices", "view_suppliers", "use_in_apu", "new_search")
}

func TestFakeAgentCreateAmbiguityAndErrorRecovery(t *testing.T) {
	engine := NewInteractionEngine(NewFakeAgent())
	assertPendingType(t, engine.Text(context.Background(), "crear material"), QuestionRequest{})
	assertPendingType(t, engine.Select(context.Background(), "conductors"), ConfirmationRequest{})
	created := engine.Select(context.Background(), "yes")
	if !hasStructuredTitle(created, "MATERIAL PREPARADO") {
		t.Fatalf("created = %#v", created.Messages)
	}
	assertPendingType(t, engine.Text(context.Background(), "ambiguo"), QuestionRequest{})
	ambiguous := engine.Select(context.Background(), "XHHW-2 · 10 AWG · Negro")
	assertStructuredWithPendingActions(t, ambiguous, "view_prices", "view_suppliers", "use_in_apu", "new_search")
	errorResponse := engine.Text(context.Background(), "error")
	if _, ok := firstMessage(errorResponse).(ErrorMessage); !ok {
		t.Fatalf("error message = %T", firstMessage(errorResponse))
	}
	assertActionValues(t, pendingActions(t, errorResponse), "retry", "new_search", "cancel")
	retry := engine.Action(context.Background(), "retry")
	assertPendingType(t, retry, QuestionRequest{})
	newSearch := engine.Action(context.Background(), "new_search")
	if newSearch.Pending != nil || !hasText(newSearch, "nueva búsqueda") {
		t.Fatalf("new search = %#v pending=%T", newSearch.Messages, newSearch.Pending)
	}
}

func TestModelSendsActionValueInsteadOfVisibleLabel(t *testing.T) {
	agent := &recordingAgent{}
	model := NewWithAgent(Handlers{}, agent)
	model, _ = update(t, model, enter())
	model, _ = update(t, model, enter())
	model, _ = submitTextModel(t, model, "fixture")
	if model.interactionMode != interactionModeAction {
		t.Fatalf("mode = %v, want action", model.interactionMode)
	}
	model, _ = update(t, model, enter())
	if agent.last.Kind != InputAction || agent.last.Value != "machine_action" {
		t.Fatalf("input = %#v, want action value", agent.last)
	}
}

func TestActionIDDispatchIgnoresLabelChanges(t *testing.T) {
	agent := &recordingAgent{}
	model := NewWithAgent(Handlers{}, agent)
	model, _ = update(t, model, enter())
	model, _ = update(t, model, enter())
	model, _ = submitTextModel(t, model, "fixture")
	model.pending = ActionRequest{Question: "Choose", Actions: []Action{{ID: "machine_action", Value: "machine_value", Label: "Changed label", Target: ActionTargetAgent}}}
	model.interactionMode = interactionModeAction
	model.syncChoiceFields()
	model, _ = update(t, model, enter())
	if agent.last.ActionID != "machine_action" || agent.last.Value != "machine_value" || agent.calls != 2 {
		t.Fatalf("input = %#v calls=%d, want stable ID/value and agent dispatch", agent.last, agent.calls)
	}
}

func TestLocalActionsDoNotCallAgent(t *testing.T) {
	agent := &recordingAgent{}
	model := NewWithAgent(Handlers{}, agent)
	model.screen = screenWorkspace
	model.pending = ActionRequest{Question: "Choose", Actions: []Action{{ID: "new_search", Value: "different-value", Label: "Changed label", Target: ActionTargetLocal}}}
	model.interactionMode = interactionModeAction
	model.syncChoiceFields()
	model, _ = update(t, model, enter())
	if agent.calls != 0 || model.pending != nil || !model.inputFocused || model.interactionMode != interactionModeChat {
		t.Fatalf("local dispatch calls=%d pending=%T focused=%v mode=%v", agent.calls, model.pending, model.inputFocused, model.interactionMode)
	}
}

func TestModelActionDockRendersActionsOnceAndPreservesIDValue(t *testing.T) {
	agent := &recordingAgent{}
	model := NewWithAgent(Handlers{}, agent)
	model, _ = update(t, model, enter())
	model, _ = update(t, model, enter())
	model, _ = submitTextModel(t, model, "fixture")
	plain := ansi.Strip(model.View().Content)
	if strings.Count(plain, "Visible label") != 1 {
		t.Fatalf("action label count = %d, want one: %q", strings.Count(plain, "Visible label"), plain)
	}
	if !strings.Contains(model.viewport.GetContent(), "Visible label") {
		t.Fatalf("pending action missing from viewport: %q", model.viewport.GetContent())
	}
	model, _ = update(t, model, enter())
	if agent.last.Kind != InputAction || agent.last.Value != "machine_action" {
		t.Fatalf("input = %#v, want action value", agent.last)
	}
}

func TestModelPendingQuestionAndConfirmationStayInDock(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pending InteractionMessage
		options []string
	}{
		{name: "question", pending: QuestionRequest{Question: "¿Qué opción?", Options: []Option{{Label: "Uno", Value: "one"}}}, options: []string{"¿Qué opción?", "Uno"}},
		{name: "confirmation", pending: ConfirmationRequest{Question: "¿Confirmar?", ConfirmLabel: "Confirmar", CancelLabel: "Cancelar"}, options: []string{"¿Confirmar?", "Confirmar", "Cancelar"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := workspaceChat(t)
			m.pending = tt.pending
			m.interactionMode = modeFor(tt.pending)
			m.syncChoiceFields()
			m.resizeViewport()
			for _, value := range tt.options {
				if !strings.Contains(m.viewport.GetContent(), value) {
					t.Fatalf("pending %q missing from viewport: %q", value, m.viewport.GetContent())
				}
			}
			dock := ansi.Strip(m.renderInteractionDock(60))
			for _, value := range tt.options {
				if strings.Contains(dock, value) {
					t.Fatalf("pending content duplicated in dock: %q", dock)
				}
			}
			if !strings.Contains(dock, "↑↓ seleccionar") {
				t.Fatalf("dock controls missing: %q", dock)
			}
		})
	}
}

func submitTextModel(t *testing.T, model Model, value string) (Model, tea.Cmd) {
	t.Helper()
	for _, char := range value {
		model, _ = update(t, model, tea.KeyPressMsg(tea.Key{Text: string(char)}))
	}
	return update(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
}

func assertPendingType(t *testing.T, response InteractionResponse, want InteractionMessage) {
	t.Helper()
	switch want.(type) {
	case QuestionRequest:
		if _, ok := response.Pending.(QuestionRequest); !ok {
			t.Fatalf("pending = %T, want question", response.Pending)
		}
	case ConfirmationRequest:
		if _, ok := response.Pending.(ConfirmationRequest); !ok {
			t.Fatalf("pending = %T, want confirmation", response.Pending)
		}
	}
}

func pendingActions(t *testing.T, response InteractionResponse) []Action {
	t.Helper()
	request, ok := response.Pending.(ActionRequest)
	if !ok {
		t.Fatalf("pending = %T, want action", response.Pending)
	}
	return request.Actions
}

func assertStructuredWithPendingActions(t *testing.T, response InteractionResponse, values ...string) {
	t.Helper()
	if _, ok := firstStructured(response); !ok {
		t.Fatalf("messages = %#v, want structured result", response.Messages)
	}
	assertActionValues(t, pendingActions(t, response), values...)
}

func assertActionValues(t *testing.T, actions []Action, values ...string) {
	t.Helper()
	if len(actions) != len(values) {
		t.Fatalf("actions = %#v, want %v", actions, values)
	}
	for i, action := range actions {
		if action.ID != values[i] || action.Value != values[i] {
			t.Fatalf("action[%d] = %#v, want ID/Value %q", i, action, values[i])
		}
	}
}

func firstMessage(response InteractionResponse) InteractionMessage {
	if len(response.Messages) == 0 {
		return nil
	}
	return response.Messages[0]
}

func firstStructured(response InteractionResponse) (StructuredResult, bool) {
	for _, message := range response.Messages {
		if result, ok := message.(StructuredResult); ok {
			return result, true
		}
	}
	return StructuredResult{}, false
}

func hasStructuredTitle(response InteractionResponse, title string) bool {
	result, ok := firstStructured(response)
	return ok && result.Title == title
}

func hasText(response InteractionResponse, want string) bool {
	for _, message := range response.Messages {
		if text, ok := message.(TextMessage); ok && containsFold(text.Text, want) {
			return true
		}
	}
	return false
}

func containsFold(value, want string) bool {
	return len(value) >= len(want) && (value == want || stringContains(value, want))
}

func stringContains(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}

type recordingAgent struct {
	last  InteractionInput
	calls int
}

func (a *recordingAgent) Respond(_ context.Context, input InteractionInput) (InteractionResponse, error) {
	a.last = input
	a.calls++
	if input.Kind == InputText {
		return InteractionResponse{Pending: ActionRequest{ID: "fixture-actions", Key: "fixture-actions", Question: "Choose", Actions: []Action{{ID: "machine_action", Value: "machine_action", Label: "Visible label", Target: ActionTargetAgent}}}}, nil
	}
	return textResponse("received"), nil
}
