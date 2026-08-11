package tui

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const (
	minWidth  = 40
	minHeight = 10
)

type screen uint8

const (
	screenHome screen = iota
	screenWorkspace
	screenLoading
	screenResult
	screenError
	screenMinSize
)

const screenMenu = screenHome

type Item struct {
	Label   string
	Handler Handler
	Quit    bool
}
type Handlers struct{ Version, Config, Status, Materials Handler }
type resultMsg struct {
	text string
	err  error
}

type Model struct {
	items         []Item
	cursor        int
	workspaceItem int
	input         string
	history       []string
	inputFocused  bool
	screen        screen
	width, height int
	result        string
}

func New(handlers Handlers) Model {
	return Model{items: []Item{
		{Label: "Materiales Maestros"},
		{Label: "Versión", Handler: handlers.Version},
		{Label: "Verificar configuración", Handler: handlers.Config},
		{Label: "Estado de GARFEX", Handler: handlers.Status},
		{Label: "Salir", Quit: true},
	}}
}
func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.width < minWidth || m.height < minHeight {
			m.screen = screenMinSize
		} else if m.screen == screenMinSize {
			m.screen = screenMenu
		}
		return m, nil
	case resultMsg:
		m.result = sanitize(msg.text)
		if msg.err != nil {
			m.result, m.screen = sanitize(msg.err.Error()), screenError
		} else {
			m.screen = screenResult
		}
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.screen == screenWorkspace && m.inputFocused {
			switch key {
			case "esc":
				m.screen = screenHome
				m.inputFocused = false
			case "enter":
				if input := strings.TrimSpace(m.input); input != "" {
					m.history = append(m.history, input)
				}
				m.input = ""
				m.screen = screenLoading
				m.inputFocused = false
				return m, simulateInput(m.historyInput())
			case "backspace":
				if len(m.input) > 0 {
					m.input = m.input[:len(m.input)-1]
				}
			default:
				if msg.Text != "" && !strings.ContainsAny(msg.Text, "\x00\x1b") {
					m.input += msg.Text
				}
			}
			return m, nil
		}
		if (m.screen == screenLoading || m.screen == screenResult || m.screen == screenError || m.screen == screenWorkspace) && key == "esc" {
			m.screen = screenHome
			m.inputFocused = false
			return m, nil
		}
		if m.screen == screenHome {
			switch key {
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
			case "enter":
				item := m.items[m.cursor]
				if item.Quit {
					return m, tea.Quit
				}
				if m.cursor == 0 {
					m.screen = screenWorkspace
					m.workspaceItem = 0
					return m, nil
				}
				m.screen = screenLoading
				if item.Handler == nil {
					return m, func() tea.Msg { return resultMsg{text: "Área disponible para una próxima entrega."} }
				}
				return m, item.Handler()
			}
		} else if m.screen == screenWorkspace {
			switch key {
			case "up":
				if m.workspaceItem > 0 {
					m.workspaceItem--
				}
			case "down":
				if m.workspaceItem < 3 {
					m.workspaceItem++
				}
			case "enter":
				if m.workspaceItem == 0 {
					m.inputFocused = true
				} else {
					m.screen = screenLoading
					return m, simulateShortcut(m.workspaceItem)
				}
			}
		} else if m.screen == screenResult || m.screen == screenError {
			if key == "enter" {
				m.screen = screenLoading
				return m, simulateInput(m.input)
			}
		}
	}
	return m, nil
}

func (m Model) historyInput() string {
	if len(m.history) == 0 {
		return ""
	}
	return m.history[len(m.history)-1]
}

func simulateInput(input string) tea.Cmd {
	return func() tea.Msg {
		if strings.Contains(strings.ToLower(input), "error") {
			return resultMsg{err: errors.New("No pude completar la búsqueda simulada.")}
		}
		if strings.TrimSpace(input) == "" {
			return resultMsg{text: "Escribí un material para iniciar la búsqueda."}
		}
		return resultMsg{text: "Encontré una coincidencia simulada para: " + sanitize(strings.TrimSpace(input))}
	}
}

func simulateShortcut(item int) tea.Cmd {
	labels := []string{"", "Flujo de creación simulado.", "Catálogo de materiales disponible.", "Historial reciente disponible."}
	return func() tea.Msg { return resultMsg{text: labels[item]} }
}

func (m Model) View() tea.View {
	if m.screen == screenMinSize {
		return tea.NewView("La terminal debe tener al menos 40x10.")
	}
	return tea.NewView(m.render())
}
