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
		if !strings.Contains(footer, part) {
			t.Fatalf("footer missing contextual hint %q: %q", part, footer)
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
