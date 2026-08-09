package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const (
	minWidth  = 40
	minHeight = 10
)

type screen uint8

const (
	screenMenu screen = iota
	screenLoading
	screenResult
	screenError
	screenMinSize
)

type Item struct {
	Label   string
	Handler Handler
}
type Handlers struct{ Version, Config, Status Handler }
type resultMsg struct {
	text string
	err  error
}

type Model struct {
	items         []Item
	cursor        int
	screen        screen
	width, height int
	result        string
}

func New(handlers Handlers) Model {
	return Model{items: []Item{{"Version", handlers.Version}, {"Config check", handlers.Config}, {"GARFEX status", handlers.Status}, {"Exit", nil}}}
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
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.screen == screenMenu {
			switch key {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
			case "enter", "space", " ":
				item := m.items[m.cursor]
				if item.Label == "Exit" {
					return m, tea.Quit
				}
				m.screen = screenLoading
				return m, item.Handler()
			}
		} else if m.screen == screenResult || m.screen == screenError {
			if key == "b" || key == "esc" {
				m.screen = screenMenu
			} else if key == "r" {
				m.screen = screenLoading
				return m, m.items[m.cursor].Handler()
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	if m.screen == screenMinSize {
		return tea.NewView("Terminal must be at least 40x10.")
	}
	if m.screen == screenLoading {
		return tea.NewView("Loading " + m.items[m.cursor].Label + "...")
	}
	if m.screen == screenResult {
		return tea.NewView(m.result + "\n\nPress b or esc to return.")
	}
	if m.screen == screenError {
		return tea.NewView(m.result + "\n\nPress r to retry, b or esc to return.")
	}
	var lines []string
	for i, item := range m.items {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		lines = append(lines, prefix+item.Label)
	}
	return tea.NewView(fmt.Sprintf("GARFEX\n\n%s\n\nUse arrows or j/k, enter or space to select.", strings.Join(lines, "\n")))
}
