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

type resourceNavigationAgent struct{ inputs []InteractionInput }

func (a *resourceNavigationAgent) Respond(_ context.Context, input InteractionInput) (InteractionResponse, error) {
	a.inputs = append(a.inputs, input)
	if input.Kind == InputText {
		return InteractionResponse{Pending: QuestionRequest{
			Key: searchResultsKey, Prompt: "Resultados", SelectionMode: SelectionSingle,
			Options: []Option{{Label: "Cable 1", Value: "MATERIAL|CABLE-1"}, {Label: "Cable 2", Value: "MATERIAL|CABLE-2"}},
		}}, nil
	}
	if input.Kind == InputSelection && input.Key == searchResultsKey {
		return InteractionResponse{
			Messages: []InteractionMessage{StructuredResult{Title: "Detalle " + input.Value}},
			Pending:  ActionRequest{Question: "Acciones", Actions: []Action{{ID: backActionID, Label: "Volver", Value: backActionID}}},
		}, nil
	}
	return InteractionResponse{}, nil
}

func directResourceNavigationModel(t *testing.T) (Model, *resourceNavigationAgent) {
	t.Helper()
	agent := &resourceNavigationAgent{}
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), []WorkspaceDescriptor{{
		Slug: "recursos", Title: "GARFEX / RECURSOS", CreateLabel: "Crear recurso", SearchOnEnter: true, Agent: agent,
	}})
	if !m.enterWorkspace("recursos") {
		t.Fatal("enterWorkspace(recursos) = false")
	}
	return m, agent
}

func navigationModel() (Model, *fakeCatalogAgent, *navigationAgent) {
	resources := &fakeCatalogAgent{}
	configuration := &navigationAgent{}
	catalog := domain.SeedResourceCatalog()
	return NewWithCatalog(Handlers{}, NewFakeAgent(), catalog, func(string) InteractionAgent {
		return resources
	}, domain.NewCatalogRegistry(), configuration), resources, configuration
}

func openResourcesSearch(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	return m
}

func ctrlN() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: 'n', Mod: tea.ModCtrl})
}

func TestResourceCatalogWorkspacesEnterFocusedSearchFromData(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	catalog.Classes = append(catalog.Classes, domain.ResourceClass{
		Code: "SERVICIO", Name: "Servicio", Plural: "Servicios", Slug: "servicios", Order: 4, Active: true,
	})
	agent := &fakeCatalogAgent{}
	descriptors := BuildWorkspaceDescriptors(catalog, func(string) InteractionAgent { return agent })
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), descriptors)

	for _, descriptor := range descriptors {
		t.Run(descriptor.Slug, func(t *testing.T) {
			candidate := m
			if !candidate.enterWorkspace(descriptor.Slug) {
				t.Fatalf("enterWorkspace(%q) = false", descriptor.Slug)
			}
			if candidate.interactionMode != interactionModeChat || !candidate.inputFocused {
				t.Fatalf("entry mode=%v focused=%v, want focused search", candidate.interactionMode, candidate.inputFocused)
			}
			plain := ansi.Strip(candidate.View().Content)
			for _, want := range []string{"› Buscar", "Ctrl+N " + descriptor.CreateLabel} {
				if !strings.Contains(plain, want) {
					t.Fatalf("view missing %q: %q", want, plain)
				}
			}
			for _, unwanted := range []string{"Buscar recursos", "Todavía no hay mensajes. Iniciá una búsqueda para comenzar."} {
				if strings.Contains(plain, unwanted) {
					t.Fatalf("view contains obsolete text %q: %q", unwanted, plain)
				}
			}
		})
	}
}

func TestResourceSearchCtrlNCreatesAndNormalLettersRemainInput(t *testing.T) {
	m, resources, _ := navigationModel()
	m = openResourcesSearch(t, m)
	for _, character := range "jkbedrxn" {
		m, _ = update(t, m, key(character))
	}
	if m.input != "jkbedrxn" || resources.calls != 0 || m.interactionMode != interactionModeChat {
		t.Fatalf("normal letters input=%q calls=%d mode=%v, want focused text input without actions", m.input, resources.calls, m.interactionMode)
	}
	m, _ = update(t, m, ctrlN())
	if resources.last.ActionID != createResourceActionID {
		t.Fatalf("Ctrl+N action = %#v, want contextual create", resources.last)
	}
}

func TestDirectResourceSearchKeyboardNavigationAndDetail(t *testing.T) {
	t.Run("escape returns to assistant", func(t *testing.T) {
		m, _ := directResourceNavigationModel(t)
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		if m.activeWorkspace != "" {
			t.Fatalf("activeWorkspace after Esc = %q, want assistant", m.activeWorkspace)
		}
	})

	t.Run("escape from results returns to focused search", func(t *testing.T) {
		m, _ := directResourceNavigationModel(t)
		for _, character := range "cable" {
			m, _ = update(t, m, key(character))
		}
		m, _ = update(t, m, enter())
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		if m.activeWorkspace != "recursos" || m.interactionMode != interactionModeChat || !m.inputFocused {
			t.Fatalf("state after results Esc: workspace=%q mode=%v focused=%v", m.activeWorkspace, m.interactionMode, m.inputFocused)
		}
	})

	t.Run("arrows move through results and enter opens detail", func(t *testing.T) {
		m, agent := directResourceNavigationModel(t)
		for _, character := range "cable" {
			m, _ = update(t, m, key(character))
		}
		m, _ = update(t, m, enter())
		if m.choiceIndex != 0 {
			t.Fatalf("initial result index = %d, want 0", m.choiceIndex)
		}

		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
		if m.choiceIndex != 1 {
			t.Fatalf("result index after Down = %d, want 1", m.choiceIndex)
		}
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
		if m.choiceIndex != 0 {
			t.Fatalf("result index after Up = %d, want 0", m.choiceIndex)
		}
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
		m, _ = update(t, m, enter())

		last := agent.inputs[len(agent.inputs)-1]
		if last.Kind != InputSelection || last.Key != searchResultsKey || last.Value != "MATERIAL|CABLE-2" {
			t.Fatalf("detail input = %#v, want the focused second result", last)
		}
		if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "Detalle MATERIAL|CABLE-2") {
			t.Fatalf("detail view = %q", plain)
		}
	})
}

func TestDirectResourceResultsPrintableCharactersRefineWithoutNavigationOrActions(t *testing.T) {
	for _, character := range "jkbedxrq1+" {
		t.Run(string(character), func(t *testing.T) {
			m, agent := directResourceNavigationModel(t)
			for _, queryCharacter := range "cable" {
				m, _ = update(t, m, key(queryCharacter))
			}
			m, _ = update(t, m, enter())
			m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
			beforeIndex, beforeInputs := m.choiceIndex, len(agent.inputs)

			m, _ = update(t, m, key(character))

			if m.choiceIndex != beforeIndex {
				t.Fatalf("choiceIndex after %q = %d, want unchanged %d", character, m.choiceIndex, beforeIndex)
			}
			if len(agent.inputs) != beforeInputs {
				t.Fatalf("agent inputs after %q = %d, want unchanged %d", character, len(agent.inputs), beforeInputs)
			}
			if m.interactionMode != interactionModeChat || !m.inputFocused || m.input != string(character) {
				t.Fatalf("search input after %q: mode=%v focused=%v input=%q", character, m.interactionMode, m.inputFocused, m.input)
			}
		})
	}
}

func TestDirectResourceResultsCtrlNStartsCreate(t *testing.T) {
	m, agent := directResourceNavigationModel(t)
	for _, character := range "cable" {
		m, _ = update(t, m, key(character))
	}
	m, _ = update(t, m, enter())
	m, _ = update(t, m, ctrlN())

	last := agent.inputs[len(agent.inputs)-1]
	if last.Kind != InputAction || last.ActionID != createResourceActionID {
		t.Fatalf("Ctrl+N from results input = %#v, want create action", last)
	}
	if m.interactionMode != interactionModeChat || !m.inputFocused {
		t.Fatalf("state after Ctrl+N response: mode=%v focused=%v, want focused search", m.interactionMode, m.inputFocused)
	}
}

func TestResourceResultGuardLeavesDetailAndAdminShortcutsUnchanged(t *testing.T) {
	t.Run("resource detail edit", func(t *testing.T) {
		m, agent := directResourceNavigationModel(t)
		m.pending = ActionRequest{Actions: []Action{{ID: editActionID, Label: "Editar", Value: editActionID, Target: ActionTargetAgent}}}
		m.interactionMode = interactionModeAction

		_, _ = update(t, m, key('e'))

		last := agent.inputs[len(agent.inputs)-1]
		if last.Kind != InputAction || last.ActionID != editActionID {
			t.Fatalf("resource detail shortcut input = %#v, want edit action", last)
		}
	})

	t.Run("catalog administration deactivate", func(t *testing.T) {
		m, _, administration := navigationModel()
		if !m.enterWorkspace(configuracionSlug) {
			t.Fatal("enterWorkspace(configuracion) = false")
		}
		m.pending = ActionRequest{Actions: []Action{{ID: catalogRecordDeactivateActionID, Label: "Desactivar", Value: catalogRecordDeactivateActionID, Target: ActionTargetAgent}}}
		m.interactionMode = interactionModeAction

		_, _ = update(t, m, key('x'))

		if administration.last.Kind != InputAction || administration.last.ActionID != catalogRecordDeactivateActionID {
			t.Fatalf("administration shortcut input = %#v, want deactivate action", administration.last)
		}
	})
}

func TestSlashRemainsAvailableFromDirectResourceSearch(t *testing.T) {
	m, _, _ := navigationModel()
	m = openResourcesSearch(t, m)
	m, _ = update(t, m, key('/'))
	if m.interactionMode != interactionModePalette {
		t.Fatalf("slash mode=%v, want command palette", m.interactionMode)
	}
}

func TestResourceCreateHintUsesCatalogLabels(t *testing.T) {
	catalog := domain.SeedResourceCatalog()
	descriptors := BuildWorkspaceDescriptors(catalog, func(string) InteractionAgent { return &fakeCatalogAgent{} })
	wants := map[string]string{
		recursosSlug: "Crear recurso", "materiales": "Crear material",
		"mano-de-obra": "Crear mano de obra", "equipo-herramienta": "Crear equipo/herramienta",
	}
	for _, descriptor := range descriptors {
		if descriptor.CreateLabel != wants[descriptor.Slug] {
			t.Fatalf("%s CreateLabel = %q, want %q", descriptor.Slug, descriptor.CreateLabel, wants[descriptor.Slug])
		}
	}
}

func TestConfigurationEntersUsefulCatalogMenuDirectly(t *testing.T) {
	m, _, configuration := navigationModel()
	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = update(t, m, enter())
	if m.activeWorkspace != configuracionSlug || m.interactionMode != interactionModeMenu {
		t.Fatalf("configuration entry workspace=%q mode=%v", m.activeWorkspace, m.interactionMode)
	}
	plain := ansi.Strip(m.View().Content)
	for _, want := range []string{"Crear estructura de recursos", "Estructura", "Características", "Unidades", "Configuración de tipos"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("configuration menu missing %q: %q", want, plain)
		}
	}
	for _, unwanted := range []string{"Catálogo de recursos", "Escribí /", "usá /"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("configuration menu contains obsolete text %q: %q", unwanted, plain)
		}
	}

	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	if configuration.last.ActionID == "" {
		t.Fatal("direct menu path did not dispatch an existing catalog capability")
	}
}
