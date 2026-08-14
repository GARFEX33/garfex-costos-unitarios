package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// fakeCatalogAgent is a minimal, dependency-free InteractionAgent standing in
// for a specialized catalog workspace's real agent (e.g.
// MaterialsWorkspaceAdapter) in tests that only need to prove *reachability*
// and *state separation*, not the real Materials search behavior — that is
// already covered by materials_workspace_adapter_test.go and left untouched.
type fakeCatalogAgent struct {
	calls int
	last  InteractionInput
}

func (a *fakeCatalogAgent) Respond(_ context.Context, input InteractionInput) (InteractionResponse, error) {
	a.calls++
	a.last = input
	return InteractionResponse{Messages: []InteractionMessage{TextMessage{Text: "materials: " + input.Value}}}, nil
}

// openMaterialsWorkspace drives the "/" palette exactly as a user would:
// opening it and confirming its first (default-focused) entry, "Materiales
// Maestros".
func openMaterialsWorkspace(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, enter())
	return m
}

func TestAssistantMainChatNeverReachesMaterialsAgent(t *testing.T) {
	materials := &fakeCatalogAgent{}
	m := NewWithAgents(Handlers{}, NewAssistantShellAgent(), materials)
	m, _ = update(t, m, enter())
	for _, char := range "cable THW-LS 10 AWG" {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	if materials.calls != 0 {
		t.Fatalf("materials agent calls = %d, want 0 (the Assistant's main chat must never reach a specialized workspace agent)", materials.calls)
	}
	if m.activeCatalog != "" {
		t.Fatalf("activeCatalog = %q, want \"\" (typing in the Assistant must not enter a catalog workspace)", m.activeCatalog)
	}
	if !historyContains(m.history, assistantShellPlaceholder) {
		t.Fatalf("history = %#v, want the Assistant's own honest placeholder response instead", m.history)
	}
}

func TestSelectingMaterialesMaestrosOpensIndependentWorkspace(t *testing.T) {
	materials := &fakeCatalogAgent{}
	m := NewWithAgents(Handlers{}, NewFakeAgent(), materials)
	m = openMaterialsWorkspace(t, m)
	if m.activeCatalog != "materials" {
		t.Fatalf("activeCatalog = %q, want %q", m.activeCatalog, "materials")
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "GARFEX / MATERIALES") {
		t.Fatalf("view = %q, want the GARFEX / MATERIALES header", plain)
	}
	m, _ = update(t, m, enter())
	for _, char := range "cable" {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	if materials.calls != 1 || materials.last.Value != "cable" {
		t.Fatalf("materials agent calls=%d last=%#v, want exactly one call with value %q", materials.calls, materials.last, "cable")
	}
}

func TestEscFromMaterialsWorkspaceReturnsToAssistant(t *testing.T) {
	m := NewWithAgents(Handlers{}, NewFakeAgent(), &fakeCatalogAgent{})
	m = openMaterialsWorkspace(t, m)
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.activeCatalog != "" {
		t.Fatalf("activeCatalog = %q, want \"\" after a single Esc from Materiales' plain chat state", m.activeCatalog)
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "GARFEX / ASSISTANT") {
		t.Fatalf("view = %q, want the GARFEX / ASSISTANT header restored", plain)
	}
}

func TestWorkspaceSwitchSeparatesStateAndPersistsMaterialsAcrossVisits(t *testing.T) {
	materials := &fakeCatalogAgent{}
	m := NewWithAgents(Handlers{}, NewFakeAgent(), materials)

	// Put text/history into the Assistant's own chat.
	m, _ = update(t, m, enter())
	for _, char := range "assistant draft" {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	if !historyContains(m.history, "assistant draft") {
		t.Fatalf("assistant history = %#v, want it to contain the assistant draft", m.history)
	}
	assistantHistoryLen := len(m.history)

	// Enter Materiales: it must start empty (first visit), never mixed with
	// the Assistant's own history.
	m = openMaterialsWorkspace(t, m)
	if len(m.history) != 0 {
		t.Fatalf("first visit history = %#v, want empty (a fresh, independent workspace)", m.history)
	}

	// Put different text/history into Materiales' own chat.
	m, _ = update(t, m, enter())
	for _, char := range "materials draft" {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	if !historyContains(m.history, "materials draft") {
		t.Fatalf("materials history = %#v, want it to contain the materials draft", m.history)
	}
	materialsHistoryLen := len(m.history)

	// Leave via Esc: the Assistant's original history/draft must be intact
	// and unmixed with Materials' own history.
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.activeCatalog != "" {
		t.Fatalf("activeCatalog = %q, want \"\" after leaving", m.activeCatalog)
	}
	if len(m.history) != assistantHistoryLen || !historyContains(m.history, "assistant draft") {
		t.Fatalf("assistant history after leaving = %#v, want the original assistant history restored unchanged", m.history)
	}
	if historyContains(m.history, "materials draft") {
		t.Fatalf("assistant history after leaving = %#v, must not contain the Materials draft", m.history)
	}

	// Re-enter Materiales via the palette again: its own history from the
	// first visit must still be there, proving materialsSaved persistence
	// across visits (not reset).
	m = openMaterialsWorkspace(t, m)
	if m.activeCatalog != "materials" {
		t.Fatalf("activeCatalog = %q, want %q on re-entry", m.activeCatalog, "materials")
	}
	if len(m.history) != materialsHistoryLen || !historyContains(m.history, "materials draft") {
		t.Fatalf("materials history on re-entry = %#v, want the persisted history from the first visit", m.history)
	}
}

// TestWorkspaceRegistrySeparatesStateAcrossTwoIndependentWorkspaces is the
// recursos-maestro PR7a RED test (design's flagged "primary D4 regression
// surface", risk R9): it proves the NEW N-workspace registry
// (WorkspaceDescriptor/workspaceSlot/Model.workspaces/enterWorkspace/
// leaveActiveWorkspace) keeps two independently-registered workspaces'
// conversations completely separate — never blended — exactly like
// TestWorkspaceSwitchSeparatesStateAndPersistsMaterialsAcrossVisits above
// already proves for the OLD single-workspace activeCatalog mechanism. It is
// written BEFORE any of those identifiers exist in model.go, so it is
// expected to fail to compile until 7a.2's GREEN step adds them (the same
// "compiler as RED" evidence class recursos-maestro PR6 already established
// for this project's mechanical/structural additions).
func TestWorkspaceRegistrySeparatesStateAcrossTwoIndependentWorkspaces(t *testing.T) {
	first := &fakeCatalogAgent{}
	second := &fakeCatalogAgent{}
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), []WorkspaceDescriptor{
		{Slug: "alpha", Title: "GARFEX / ALPHA", CreateLabel: "Crear alpha", Agent: first},
		{Slug: "beta", Title: "GARFEX / BETA", CreateLabel: "Crear beta", Agent: second},
	})

	if ok := m.enterWorkspace("alpha"); !ok {
		t.Fatal("enterWorkspace(\"alpha\") = false, want true")
	}
	if m.activeWorkspace != "alpha" {
		t.Fatalf("activeWorkspace = %q, want %q", m.activeWorkspace, "alpha")
	}
	m = submitText(t, m, "alpha draft")
	if !historyContains(m.history, "alpha draft") {
		t.Fatalf("alpha history = %#v, want it to contain the alpha draft", m.history)
	}
	alphaHistoryLen := len(m.history)

	// Leave alpha and enter beta: beta's first visit must start completely
	// empty, never mixed with alpha's own history.
	m.leaveActiveWorkspace()
	if m.activeWorkspace != "" {
		t.Fatalf("activeWorkspace after leaving = %q, want \"\"", m.activeWorkspace)
	}
	if ok := m.enterWorkspace("beta"); !ok {
		t.Fatal("enterWorkspace(\"beta\") = false, want true")
	}
	if len(m.history) != 0 {
		t.Fatalf("beta first-visit history = %#v, want empty (a fresh, independent workspace)", m.history)
	}
	m = submitText(t, m, "beta draft")
	if !historyContains(m.history, "beta draft") {
		t.Fatalf("beta history = %#v, want it to contain the beta draft", m.history)
	}
	if historyContains(m.history, "alpha draft") {
		t.Fatalf("beta history = %#v, must not contain the alpha draft (state separation)", m.history)
	}

	// Leave beta and re-enter alpha: alpha's own history from the first
	// visit must still be there, unmixed with beta's — proving
	// workspaceSlot.saved persistence AND separation, not a shared/blended
	// snapshot.
	m.leaveActiveWorkspace()
	if ok := m.enterWorkspace("alpha"); !ok {
		t.Fatal("re-entering \"alpha\" = false, want true")
	}
	if len(m.history) != alphaHistoryLen || !historyContains(m.history, "alpha draft") {
		t.Fatalf("alpha history on re-entry = %#v, want the persisted alpha history from the first visit", m.history)
	}
	if historyContains(m.history, "beta draft") {
		t.Fatalf("alpha history on re-entry = %#v, must not contain the beta draft", m.history)
	}

	// An unknown slug must leave every field untouched.
	if ok := m.enterWorkspace("does-not-exist"); ok {
		t.Fatal("enterWorkspace(unknown slug) = true, want false")
	}
	if m.activeWorkspace != "alpha" {
		t.Fatalf("activeWorkspace after an unknown enterWorkspace call = %q, want it untouched (%q)", m.activeWorkspace, "alpha")
	}
}
