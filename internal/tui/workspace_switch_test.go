package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// fakeCatalogAgent is a minimal, dependency-free InteractionAgent standing in
// for a specialized workspace's real agent (e.g. ResourcesWorkspaceAdapter)
// in tests that only need to prove *reachability* and *state separation*,
// not the real search behavior — that is already covered by
// resources_workspace_adapter_test.go and left untouched.
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

// materialsWorkspaceDescriptors builds the single-entry registry these tests
// need: a "materials" slot whose Slug matches the "materials" palette action
// id already declared in commands.go's assistantActions (unchanged by this
// PR — PR8's job), so openMaterialsWorkspace's real palette-selection flow
// reaches this slot through the registry mechanism (registry-based literal
// sites, design §6), not the retired activeCatalog/materialsAgent mechanism.
func materialsWorkspaceDescriptors(agent InteractionAgent) []WorkspaceDescriptor {
	return []WorkspaceDescriptor{
		{Slug: "materials", Title: "GARFEX / MATERIALES", CreateLabel: "Crear material", Agent: agent},
	}
}

func TestAssistantMainChatNeverReachesMaterialsAgent(t *testing.T) {
	materials := &fakeCatalogAgent{}
	m := NewWithWorkspaces(Handlers{}, NewAssistantShellAgent(), materialsWorkspaceDescriptors(materials))
	m, _ = update(t, m, enter())
	for _, char := range "cable THW-LS 10 AWG" {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	if materials.calls != 0 {
		t.Fatalf("materials agent calls = %d, want 0 (the Assistant's main chat must never reach a specialized workspace agent)", materials.calls)
	}
	if m.activeWorkspace != "" {
		t.Fatalf("activeWorkspace = %q, want \"\" (typing in the Assistant must not enter a registered workspace)", m.activeWorkspace)
	}
	if !historyContains(m.history, assistantShellPlaceholder) {
		t.Fatalf("history = %#v, want the Assistant's own honest placeholder response instead", m.history)
	}
}

func TestSelectingMaterialesMaestrosOpensIndependentWorkspace(t *testing.T) {
	materials := &fakeCatalogAgent{}
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), materialsWorkspaceDescriptors(materials))
	m = openMaterialsWorkspace(t, m)
	if m.activeWorkspace != "materials" {
		t.Fatalf("activeWorkspace = %q, want %q", m.activeWorkspace, "materials")
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "GARFEX › MATERIALES › Menú") {
		t.Fatalf("view = %q, want the Materiales menu breadcrumb", plain)
	}
	m, _ = update(t, m, key('b'))
	for _, char := range "cable" {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	if materials.calls != 1 || materials.last.Value != "cable" {
		t.Fatalf("materials agent calls=%d last=%#v, want exactly one call with value %q", materials.calls, materials.last, "cable")
	}
}

func TestEscFromMaterialsWorkspaceReturnsToAssistant(t *testing.T) {
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), materialsWorkspaceDescriptors(&fakeCatalogAgent{}))
	m = openMaterialsWorkspace(t, m)
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.activeWorkspace != "" {
		t.Fatalf("activeWorkspace = %q, want \"\" after a single Esc from Materiales' plain chat state", m.activeWorkspace)
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "GARFEX / ASSISTANT") {
		t.Fatalf("view = %q, want the GARFEX / ASSISTANT header restored", plain)
	}
}

func TestWorkspaceSwitchSeparatesStateAndPersistsMaterialsAcrossVisits(t *testing.T) {
	materials := &fakeCatalogAgent{}
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), materialsWorkspaceDescriptors(materials))

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
	m, _ = update(t, m, key('b'))
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
	if m.activeWorkspace != "" {
		t.Fatalf("activeWorkspace = %q, want \"\" after leaving", m.activeWorkspace)
	}
	if len(m.history) != assistantHistoryLen || !historyContains(m.history, "assistant draft") {
		t.Fatalf("assistant history after leaving = %#v, want the original assistant history restored unchanged", m.history)
	}
	if historyContains(m.history, "materials draft") {
		t.Fatalf("assistant history after leaving = %#v, must not contain the Materials draft", m.history)
	}

	// Re-enter Materiales via the palette again: its own history from the
	// first visit must still be there, proving workspaceSlot.saved
	// persistence across visits (not reset).
	m = openMaterialsWorkspace(t, m)
	if m.activeWorkspace != "materials" {
		t.Fatalf("activeWorkspace = %q, want %q on re-entry", m.activeWorkspace, "materials")
	}
	if len(m.history) != materialsHistoryLen || !historyContains(m.history, "materials draft") {
		t.Fatalf("materials history on re-entry = %#v, want the persisted history from the first visit", m.history)
	}
}

// TestActivePaletteActionsIsWorkspaceSlugGeneric is a PR7b RED test for the
// activePaletteActions literal site (design §6 table row 1): it proves the
// "/" palette switches away from the global assistantActions tree for ANY
// registered workspace slug, not a hardcoded "materials" string comparison —
// the mechanism this PR introduces, ahead of PR8's per-descriptor
// workspaceActions builder (which will replace the shared materialsActions
// var with a real per-slug tree). A workspace named "beta" (never mentioned
// anywhere in commands.go) reaching this behavior is the proof the check is
// no longer literal-bound.
func TestActivePaletteActionsIsWorkspaceSlugGeneric(t *testing.T) {
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), []WorkspaceDescriptor{
		{Slug: "beta", Title: "GARFEX / BETA", CreateLabel: "Crear beta", Agent: &fakeCatalogAgent{}},
	})
	if ok := m.enterWorkspace("beta"); !ok {
		t.Fatal("enterWorkspace(\"beta\") = false, want true")
	}
	actions := m.activePaletteActions()
	for _, action := range actions {
		if action.label == "Materiales Maestros" {
			t.Fatalf("activePaletteActions() while inside \"beta\" = %#v, must not show the global assistantActions tree", actions)
		}
	}
}

// TestEscFromRegisteredWorkspacePlainChatReturnsToAssistant is a PR7b RED
// test for the Esc-handling literal site (design §6 table row 4): entering a
// workspace directly through the registry (m.enterWorkspace, NOT the
// palette's still-materials-only confirm path) and pressing a plain Esc from
// its chat state must return to the Assistant — proving the check reads
// activeWorkspace/leaveActiveWorkspace, not the retired
// activeCatalog/leaveActiveCatalog pair, which this scenario would never
// touch (activeCatalog stays "" the whole time since enterWorkspace, not
// enterMaterialsWorkspace, was used to enter).
func TestEscFromRegisteredWorkspacePlainChatReturnsToAssistant(t *testing.T) {
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), []WorkspaceDescriptor{
		{Slug: "beta", Title: "GARFEX / BETA", CreateLabel: "Crear beta", Agent: &fakeCatalogAgent{}},
	})
	// heroActive suppresses the header line regardless of workspace (see
	// renderWorkspace); disable it first, exactly as the real "/" keypress
	// path already does before enterWorkspace ever snapshots the Assistant's
	// state, so this test isolates the Esc-handling literal site only.
	m.heroActive = false
	if ok := m.enterWorkspace("beta"); !ok {
		t.Fatal("enterWorkspace(\"beta\") = false, want true")
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.activeWorkspace != "" {
		t.Fatalf("activeWorkspace = %q, want \"\" after Esc from a registered workspace's plain chat state", m.activeWorkspace)
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "GARFEX / ASSISTANT") {
		t.Fatalf("view = %q, want the GARFEX / ASSISTANT header restored", plain)
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
	m, _ = update(t, m, key('b'))
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
	m, _ = update(t, m, key('b'))
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
