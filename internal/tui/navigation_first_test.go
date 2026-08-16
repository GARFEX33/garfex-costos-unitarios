package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/charmbracelet/x/ansi"
)

type navigationAgent struct{ last InteractionInput }

func (a *navigationAgent) Respond(_ context.Context, input InteractionInput) (InteractionResponse, error) {
	a.last = input
	return InteractionResponse{Pending: QuestionRequest{Key: catalogStatusMenuKey, Prompt: "Elegí un estado", SelectionMode: SelectionSingle, Options: []Option{{ID: "active", Label: "Activos", Value: "active"}}}}, nil
}

func navigationModel() (Model, *fakeCatalogAgent, *navigationAgent) {
	resources := &fakeCatalogAgent{}
	configuration := &navigationAgent{}
	catalog := domain.SeedResourceCatalog()
	return NewWithCatalog(Handlers{}, NewFakeAgent(), catalog, func(string) InteractionAgent {
		return resources
	}, domain.NewCatalogRegistry(), configuration), resources, configuration
}

func openResourcesMenu(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	return m
}

func TestResourcesEnterNavigationAndEscapeWithoutPermanentComposer(t *testing.T) {
	m, _, _ := navigationModel()
	m = openResourcesMenu(t, m)
	plain := ansi.Strip(m.View().Content)
	for _, want := range []string{"GARFEX › RECURSOS › Menú", "Crear recurso", "Buscar recursos"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("resources menu missing %q: %q", want, plain)
		}
	}
	if strings.Contains(plain, "Pregúntame") || strings.Contains(plain, "❯ /") {
		t.Fatalf("administrative menu rendered a conversational composer: %q", plain)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = update(t, m, enter())
	if m.interactionMode != interactionModeChat || !m.inputFocused {
		t.Fatalf("Buscar selection mode=%v focused=%v", m.interactionMode, m.inputFocused)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.interactionMode != interactionModeMenu {
		t.Fatalf("Esc from search mode=%v, want menu", m.interactionMode)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.activeWorkspace != "" {
		t.Fatalf("Esc from root menu left workspace %q active", m.activeWorkspace)
	}
}

func TestAdministrativeShortcutsAndContextualPaletteOnlyRunAvailableActions(t *testing.T) {
	m, resources, _ := navigationModel()
	m = openResourcesMenu(t, m)
	m, _ = update(t, m, key('+'))
	if resources.last.ActionID != createResourceActionID {
		t.Fatalf("+ action = %#v, want create", resources.last)
	}
	m.pending = ActionRequest{Actions: []Action{{ID: editActionID, Label: "Editar", Value: editActionID, Target: ActionTargetAgent}}}
	m.interactionMode = interactionModeAction
	m, _ = update(t, m, key('/'))
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, "Editar") || strings.Contains(got, "Duplicar") {
		t.Fatalf("contextual palette = %q", got)
	}
	m, _ = update(t, m, enter())
	if resources.last.ActionID != editActionID {
		t.Fatalf("contextual action = %#v, want edit", resources.last)
	}
	m, _ = update(t, m, key('?'))
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, "b buscar") {
		t.Fatalf("? did not reveal contextual help: %q", got)
	}
}

func TestConfigurationEntersNavigableMenuAndList(t *testing.T) {
	m, _, configuration := navigationModel()
	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = update(t, m, enter())
	if m.activeWorkspace != configuracionSlug || m.interactionMode != interactionModeMenu {
		t.Fatalf("configuration entry workspace=%q mode=%v", m.activeWorkspace, m.interactionMode)
	}
	m, _ = update(t, m, enter())
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	if configuration.last.ActionID == "" {
		t.Fatal("menu path did not dispatch an existing catalog capability")
	}
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, "› Lista") || strings.Contains(got, "Pregúntame") {
		t.Fatalf("configuration list structure = %q", got)
	}
}
