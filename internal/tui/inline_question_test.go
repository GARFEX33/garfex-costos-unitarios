package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Phase 1/2 — Option.Description rendering.

func TestOptionDescriptionShownWhenPresent(t *testing.T) {
	request := QuestionRequest{
		ID:            "insulation",
		Key:           "insulation",
		Prompt:        "¿Qué aislamiento necesitas?",
		SelectionMode: SelectionSingle,
		Options: []Option{
			{Label: "THW-LS", Value: "thw-ls", Description: "Resistente al calor y la humedad"},
		},
	}
	plain := ansi.Strip(renderActiveInteractionWithSelection(request, 0, nil))
	if !strings.Contains(plain, "Resistente al calor y la humedad") {
		t.Fatalf("expected option description rendered, got %q", plain)
	}
}

func TestOptionDescriptionOmittedWhenEmpty(t *testing.T) {
	request := QuestionRequest{
		ID:            "insulation",
		Key:           "insulation",
		Prompt:        "¿Qué aislamiento necesitas?",
		SelectionMode: SelectionSingle,
		Options: []Option{
			{Label: "THW-LS", Value: "thw-ls"},
		},
	}
	plain := ansi.Strip(renderActiveInteractionWithSelection(request, 0, nil))
	lines := strings.Split(plain, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected prompt + single option line only when description is empty, got %d lines: %#v", len(lines), lines)
	}
}

func TestMultipleOptionDescriptionShownWhenPresent(t *testing.T) {
	got := renderMultipleOption(Option{Label: "THW-LS", Value: "thw-ls", Description: "Resistente al calor y la humedad"}, false, false)
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "Resistente al calor y la humedad") {
		t.Fatalf("expected multiple-select option description rendered, got %q", plain)
	}
}

func TestMultipleOptionDescriptionOmittedWhenEmpty(t *testing.T) {
	got := renderMultipleOption(Option{Label: "THW-LS", Value: "thw-ls"}, false, false)
	plain := ansi.Strip(got)
	if strings.Contains(plain, "\n") {
		t.Fatalf("expected a single line when description is empty, got %q", plain)
	}
}

// Phase 1/2 — resolvedInteraction.cancelled and discrete cancellation mark.

func TestCancelledInteractionRendersDiscreteMark(t *testing.T) {
	got := renderResolvedInteraction(resolvedInteraction{prompt: "¿Qué aislamiento necesitas?", cancelled: true})
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "· cancelado") {
		t.Fatalf("expected discrete cancellation mark, got %q", plain)
	}
	if strings.Contains(plain, "✓") {
		t.Fatalf("cancelled interaction must not render the answered check mark: %q", plain)
	}
}

func TestAnsweredInteractionRendersResolvedMark(t *testing.T) {
	got := renderResolvedInteraction(resolvedInteraction{prompt: "¿Qué aislamiento necesitas?", selection: "THW-LS", cancelled: false})
	if got != "¿Qué aislamiento necesitas?\n✓ THW-LS" {
		t.Fatalf("resolved shape changed: %q", got)
	}
}

func TestEscCancelsInlineQuestionMarksDiscrete(t *testing.T) {
	m := submitText(t, workspaceChat(t), "cable 10")
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	plain := ansi.Strip(m.viewport.GetContent())
	if !strings.Contains(plain, "¿Qué aislamiento necesitas?\n· cancelado") {
		t.Fatalf("expected discrete cancellation mark after esc, got %q", plain)
	}
}

func TestEscCancelsSearchableInlineQuestionMarksDiscrete(t *testing.T) {
	m := submitText(t, workspaceChat(t), "buscar una opción")
	if m.interactionMode != interactionModeSearchable {
		t.Fatalf("expected searchable pending question, got mode %v pending %T", m.interactionMode, m.pending)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	plain := ansi.Strip(m.viewport.GetContent())
	if !strings.Contains(plain, "· cancelado") {
		t.Fatalf("expected discrete cancellation mark after esc on searchable question, got %q", plain)
	}
}

func TestEscInManualSearchDoesNotAppendHistory(t *testing.T) {
	m := New(Handlers{})
	prior := len(m.history)
	m.openManualSearch()
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if len(m.history) != prior {
		t.Fatalf("manual search cancel must not add a history entry (out of scope for this PR): history=%d want=%d", len(m.history), prior)
	}
}

// Phase 3 — composer stays visible and contextual help footer.

func TestComposerVisibleDuringPendingQuestion(t *testing.T) {
	tests := []struct {
		name  string
		model func(t *testing.T) Model
	}{
		{name: "choice", model: func(t *testing.T) Model { return submitText(t, workspaceChat(t), "cable 10") }},
		{name: "multiple", model: func(t *testing.T) Model { return submitText(t, workspaceChat(t), "buscar material") }},
		{name: "searchable", model: func(t *testing.T) Model { return submitText(t, workspaceChat(t), "buscar una opción") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.model(t)
			dock := ansi.Strip(m.renderInteractionDock(60))
			if !strings.Contains(dock, "❯ ") {
				t.Fatalf("composer prompt missing while question pending: %q", dock)
			}
		})
	}
}

func TestContextualHelpFooterMatchesPendingQuestion(t *testing.T) {
	m := submitText(t, workspaceChat(t), "cable 10")
	footer := ansi.Strip(m.renderFooter(60))
	for _, part := range questionHelpParts(m.pending, false) {
		if !strings.Contains(footer, part.Description) {
			t.Fatalf("footer missing contextual hint %q: %q", part.Description, footer)
		}
	}
}

func TestContextualHelpNotDuplicated(t *testing.T) {
	m := submitText(t, workspaceChat(t), "cable 10")
	dock := ansi.Strip(m.renderInteractionDock(60))
	if count := strings.Count(dock, "seleccionar"); count != 1 {
		t.Fatalf("dock hint text duplicated %d times: %q", count, dock)
	}
}

func TestGeneralHelpRestoredWhenNoPending(t *testing.T) {
	m := workspaceChat(t)
	if m.pending != nil {
		t.Fatalf("expected no pending question, got %#v", m.pending)
	}
	footer := ansi.Strip(m.renderFooter(60))
	if strings.Contains(footer, "seleccionar") {
		t.Fatalf("general footer should not show question hints: %q", footer)
	}
}

// Phase 4 — AllowCustom free text + dual Enter.

// customGaugeQuestion drives the FakeAgent's "custom-gauge" fixture
// scenario (AllowCustom: true, searchable, two Options with Description)
// through the normal chat submit path.
func customGaugeQuestion(t *testing.T) Model {
	t.Helper()
	m := submitText(t, workspaceChat(t), "calibre personalizado")
	if m.interactionMode != interactionModeSearchable {
		t.Fatalf("expected searchable AllowCustom pending question, got mode %v pending %T", m.interactionMode, m.pending)
	}
	request, ok := m.pending.(QuestionRequest)
	if !ok || !request.AllowCustom {
		t.Fatalf("expected AllowCustom pending question, got %#v", m.pending)
	}
	return m
}

// allowCustomChoiceModel builds a single-select (non-searchable) AllowCustom
// pending question directly, mirroring the searchableModel test helper.
func allowCustomChoiceModel(agent InteractionAgent) Model {
	m := NewWithAgent(Handlers{}, agent)
	m.screen = screenWorkspace
	m.pending = QuestionRequest{
		Key:           "gauge",
		Prompt:        "Indica el calibre (o escribe uno personalizado)",
		SelectionMode: SelectionSingle,
		AllowCustom:   true,
		Options:       customGaugeOptions(),
	}
	m.interactionMode = interactionModeChoice
	m.syncChoiceFields()
	m.resizeViewport()
	return m
}

// allowCustomSearchableModel builds a searchable AllowCustom pending
// question directly, mirroring the searchableModel test helper.
func allowCustomSearchableModel(agent InteractionAgent) Model {
	m := NewWithAgent(Handlers{}, agent)
	m.screen = screenWorkspace
	m.pending = QuestionRequest{
		Key:           "gauge",
		Prompt:        "Indica el calibre (o escribe uno personalizado)",
		SelectionMode: SelectionSearchable,
		AllowCustom:   true,
		Options:       customGaugeOptions(),
	}
	m.interactionMode = interactionModeSearchable
	m.syncChoiceFields()
	m.resizeViewport()
	return m
}

func TestAllowCustomLiteralJKKeys(t *testing.T) {
	tests := []struct {
		name string
		key  rune
	}{
		{name: "j is typed, not navigation", key: 'j'},
		{name: "k is typed, not navigation", key: 'k'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := customGaugeQuestion(t)
			startIndex := m.choiceIndex
			m, _ = update(t, m, key(tt.key))
			if m.choiceIndex != startIndex {
				t.Fatalf("expected focus unchanged for literal %q, got index %d want %d", tt.key, m.choiceIndex, startIndex)
			}
			if m.input != string(tt.key) {
				t.Fatalf("expected %q appended to m.input, got %q", tt.key, m.input)
			}
		})
	}
}

func TestAllowCustomArrowsStillNavigate(t *testing.T) {
	m := customGaugeQuestion(t)
	if m.choiceIndex != 0 {
		t.Fatalf("expected starting focus at 0, got %d", m.choiceIndex)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.choiceIndex != 1 {
		t.Fatalf("expected down arrow to move focus, got %d", m.choiceIndex)
	}
	if m.input != "" {
		t.Fatalf("expected arrow navigation to leave input text unchanged, got %q", m.input)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.choiceIndex != 0 {
		t.Fatalf("expected up arrow to move focus back, got %d", m.choiceIndex)
	}
}

func TestAllowCustomTypingAccumulatesInput(t *testing.T) {
	m := customGaugeQuestion(t)
	for _, char := range "16 AWG" {
		m, _ = update(t, m, key(char))
	}
	if m.input != "16 AWG" {
		t.Fatalf("expected typed text to accumulate in m.input, got %q", m.input)
	}
	if m.choiceIndex != 0 {
		t.Fatalf("expected focus unchanged while typing, got index %d", m.choiceIndex)
	}
}

func TestDualEnterConfirmsFocusedOptionWhenInputEmpty(t *testing.T) {
	tests := []struct {
		name  string
		model func(agent InteractionAgent) Model
	}{
		{name: "single-select", model: allowCustomChoiceModel},
		{name: "searchable", model: allowCustomSearchableModel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &recordingAgent{}
			m := tt.model(agent)
			m.choiceIndex = 1
			m, _ = update(t, m, enter())
			if agent.last.Kind != InputSelection || agent.last.Key != "gauge" || agent.last.Value != "12awg" {
				t.Fatalf("expected focused option submitted when input is empty, got %#v", agent.last)
			}
		})
	}
}

// The searchable-select case documents an intentional, flagged UX
// follow-up from the spec: when AllowCustom is true and the input is
// non-empty, the typed text wins over the visually-focused *filtered*
// option, even if that focused option is still visible. This is the
// approved literal spec wording (Dual Enter Behavior), not an
// accidental side effect of sharing m.input as both the filter query
// and the free-text candidate. Do not redesign; only revisit under an
// explicit new requirement.
func TestDualEnterSubmitsCustomTextWhenInputNonEmpty(t *testing.T) {
	tests := []struct {
		name  string
		model func(agent InteractionAgent) Model
	}{
		{name: "single-select", model: allowCustomChoiceModel},
		{name: "searchable", model: allowCustomSearchableModel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &recordingAgent{}
			m := tt.model(agent)
			m.choiceIndex = 1 // focused option ("12awg") must be ignored in favor of typed text
			m.input = "16 AWG"
			m, _ = update(t, m, enter())
			if agent.last.Kind != InputSelection || agent.last.Key != "gauge" || agent.last.Value != "16 AWG" {
				t.Fatalf("expected typed text submitted over focused option, got %#v", agent.last)
			}
			if m.input != "" {
				t.Fatalf("expected input cleared after submitting custom text, got %q", m.input)
			}
		})
	}
}

// TestAllowCustomSearchableRenderFiltersByInput proves the render call site
// (refreshViewport) uses the same query source as pendingOptions()/
// syncChoiceFields() when AllowCustom is true: m.input, not the never-written
// m.searchQuery. Regression test for the render/selection query mismatch bug.
func TestAllowCustomSearchableRenderFiltersByInput(t *testing.T) {
	m := customGaugeQuestion(t)
	for _, char := range "10" {
		m, _ = update(t, m, key(char))
	}
	if m.input != "10" {
		t.Fatalf("expected typed text to accumulate in m.input, got %q", m.input)
	}
	m.refreshViewport()
	plain := ansi.Strip(m.viewport.GetContent())
	if !strings.Contains(plain, "Buscar: 10") {
		t.Fatalf("expected rendered search line to reflect typed input %q, got %q", m.input, plain)
	}
	if !strings.Contains(plain, "10 AWG") {
		t.Fatalf("expected matching option %q still rendered, got %q", "10 AWG", plain)
	}
	if strings.Contains(plain, "12 AWG") {
		t.Fatalf("expected non-matching option %q filtered out of render, got %q", "12 AWG", plain)
	}
}
