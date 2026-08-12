package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	defaultWidth  = 72
	defaultHeight = 20
	maxCardWidth  = 100

	officialTagline  = "DISEÑO · CONSTRUCCIÓN · MANTENIMIENTO ELÉCTRICO"
	backgroundHex    = "#0B0D0E"
	surfaceHex       = "#17191B"
	primaryTextHex   = "#F2F0E9"
	secondaryTextHex = "#8B8B86"
	brandRedHex      = "#800000"
	accentHex        = "#FFD400"
	successHex       = "#4FC38A"
	errorHex         = "#FF6B6B"

	fullWordmark = `⠀⠀⠀⡄⡘⠤⡉⠌⣾⣿⣿⣶⣄⠀⠀⠀⠀⠀⠀⠀⠀⣼⣿⣿⣿⣿⣷⠀⠀⠀⠀⠀⠀⠀⢘⣿⣿⣿⣿⣿⣿⣿⣿⣷⣶⣄⠀⠀⠀⠀⠀⣽⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡗⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠹⣿⣿⣿⣿⣷⡄⠀⠀⠀⣼⣿⣿⣿⣿⡿⠁⠀⠀⠀
⠀⡤⠘⡠⢑⠢⡑⣸⣿⣿⣿⡿⠋⠀⠀⠀⠀⠀⠀⠀⢰⣿⣿⣿⣿⣿⣿⣇⠀⠀⠀⠀⠀⠀⢨⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⡀⠀⠀⠀⢾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣏⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⠘⢿⣿⣿⣿⣿⣆⠠⣽⣿⣿⣿⣿⡟⠁⠀⠀⠀⠀
⣰⡇⠁⢆⠡⢆⠁⠉⠉⠙⠏⠁⠀⠀⠀⠀⠀⠀⠀⢠⣿⣿⣿⣿⣿⣿⣿⣿⡆⠀⠀⠀⠀⠀⠰⣿⣿⣿⣿⡏⠉⠉⠛⣿⣿⣿⣿⡇⠀⠀⠀⣻⣿⣿⣿⣿⠉⠉⠉⠉⠉⠉⠃⠀⠀⠀⣿⣿⣿⣿⡿⠉⠉⠉⠉⠉⠙⠁⠀⠀⠀⠀⠈⢻⣿⣿⣿⣿⣾⣿⣿⣿⣿⠏⠀⠀⠀⠀⠀⠀
⣿⠠⠑⡌⠒⡌⠒⠄⣶⣶⣶⣶⣶⣶⣶⣶⠀⠀⠀⣾⣿⣿⣿⣿⢻⣿⣿⣿⣿⡀⠀⠀⠀⠀⢘⣿⣿⣿⣿⣧⣴⣤⣾⣿⣿⣿⣿⡇⠀⠀⠀⣽⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⠀⠀⠀⠻⣿⣿⣿⣿⣿⣿⡿⠃⠀⠀⠀⠀⠀⠀⠀
⣏⣐⣁⠢⡑⢌⠁⠀⣿⣿⣿⣿⣿⣿⣿⡟⠀⠀⣸⣿⣿⣿⣿⣇⣘⣿⣿⣿⣿⣷⠀⠀⠀⠀⢨⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠟⠀⠀⠀⠀⢾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⠀⠀⠀⢤⣿⣿⣿⣿⣿⣿⣧⡀⠀⠀⠀⠀⠀⠀⠀
⢻⣿⡇⢂⠱⢈⡀⠀⠿⠿⣿⣿⣿⣿⣿⡇⠀⢰⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣆⠀⠀⠀⠰⣿⣿⣿⣿⡿⢿⣿⣿⣿⣿⡅⠀⠀⠀⠀⠀⣻⣿⣿⣿⣿⠉⠉⠉⠉⠉⠉⠁⠀⠀⠀⣿⣿⣿⣿⡿⠉⠉⠉⠉⠉⠉⠁⠀⠀⠀⠀⠀⣼⣿⣿⣿⣿⣿⣿⣿⣿⣷⡄⠀⠀⠀⠀⠀⠀
⠈⢿⠡⢌⢂⣿⣿⣷⣾⣾⣿⣿⣿⣿⡟⠀⢀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡄⠀⠀⢘⣿⣿⣿⣿⡇⠈⢿⣿⣿⣿⣿⣄⠀⠀⠀⠀⣽⣿⣿⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⢀⣾⣿⣿⣿⣿⠟⠘⢿⣿⣿⣿⣿⣆⠀⠀⠀⠀⠀
⠀⠀⡘⢠⣾⣿⣿⣿⣿⣿⣿⣿⡿⠋⠀⠀⣼⣿⣿⣿⣿⡏⠁⠉⠈⠁⠙⣿⣿⣿⣿⣷⡀⠀⢨⣿⣿⣿⣿⡇⠀⠈⢿⣿⣿⣿⣿⣆⠀⠀⠀⢾⣿⣿⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⣠⣿⣿⣿⣿⣿⠋⠀⠀⠈⢻⣿⣿⣿⣿⣧⡀⠀⠀⠀
⠀⠀⠀⠀⠈⠉⠛⠛⠛⠋⠉⠁⠀⠀⠀⠐⠉⠋⠙⠉⠙⠀⠀⠀⠀⠀⠀⠉⠋⠙⠉⠋⠁⠀⠀⠋⠙⠉⠋⠁⠀⠀⠈⠋⠙⠉⠋⠙⠀⠀⠀⠉⠋⠙⠉⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠋⠙⠉⠋⠙⠉⠋⠉⠉⠉⠙⠁⠀⠀⠋⠙⠉⠋⠙⠁⠀⠀⠀⠀⠀⠙⠉⠋⠙⠉⠃⠀⠀⠀`
)

var workspaceShortcuts = []Option{
	{ID: "new_search", Label: "Buscar material", Target: ActionTargetLocal},
	{ID: "create_material", Label: "Crear material", Target: ActionTargetLocal},
	{ID: "view_catalog", Label: "Catálogo", Target: ActionTargetLocal},
	{ID: "view_history", Label: "Historial", Target: ActionTargetLocal},
}

var (
	background    = lipgloss.Color(backgroundHex)
	surface       = lipgloss.Color(surfaceHex)
	primaryText   = lipgloss.Color(primaryTextHex)
	secondaryText = lipgloss.Color(secondaryTextHex)
	brandRed      = lipgloss.Color(brandRedHex)
	accent        = lipgloss.Color(accentHex)
	successColor  = lipgloss.Color(successHex)
)

func (m Model) render() string {
	width, height := m.width, m.height
	if width == 0 {
		width = defaultWidth
	}
	if height == 0 {
		height = defaultHeight
	}

	cardWidth := min(width-2, maxCardWidth)
	full := cardWidth >= lipgloss.Width(fullWordmark)
	if full {
		full = lipgloss.Height(m.renderCard(cardWidth, true)) <= height-2
	}
	card := m.renderCard(cardWidth, full)
	canvas := lipgloss.NewStyle().Background(background)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card,
		lipgloss.WithWhitespaceStyle(canvas))
}

func (m Model) renderCard(width int, full bool) string {
	sections := make([]string, 0, 4)
	if m.screen != screenWorkspace {
		sections = append(sections, renderWordmark(width, full), renderTagline(width), renderDivider(width, full))
	}
	if m.screen == screenHome {
		if full {
			sections = append(sections, lipgloss.NewStyle().Width(width).Foreground(secondaryText).Render("HOME · ÁREAS DE GARFEX"))
		}
		sections = append(sections, m.renderMenu(width))
	} else if m.screen == screenWorkspace {
		sections = append(sections, m.renderWorkspace(width))
	} else {
		sections = append(sections, m.renderState(width))
	}
	sections = append(sections, m.renderFooter(width))
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderWordmark(width int, full bool) string {
	wordmark := "GARFEX"
	if full {
		wordmark = fullWordmark
	}
	style := wordmarkStyle(width)
	if full {
		style = style.PaddingBottom(1)
	}
	return style.Render(wordmark)
}

func wordmarkStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Foreground(brandRed).Bold(true)
}

func renderTagline(width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Foreground(secondaryText).
		Render(officialTagline)
}

func renderDivider(width int, full bool) string {
	style := lipgloss.NewStyle().Width(width).Foreground(surface)
	if full {
		style = style.PaddingBottom(1)
	}
	return style.Render(strings.Repeat("-", width))
}

func menuItemStyle(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(primaryText)
	if active {
		return style.Foreground(accent).Bold(true)
	}
	return style
}

func interactionOptionStyle(focused, selected bool) lipgloss.Style {
	if focused {
		return lipgloss.NewStyle().Foreground(accent).Background(surface).Bold(true).Padding(0, 1)
	}
	if selected {
		return lipgloss.NewStyle().Foreground(successColor).Bold(true).Padding(0, 1)
	}
	return lipgloss.NewStyle().Foreground(secondaryText).Padding(0, 1)
}

func (m Model) renderMenu(width int) string {
	lines := make([]string, 0, len(m.items))
	for i, item := range m.items {
		active := i == m.cursor
		marker := "  "
		if active {
			marker = "› "
		}
		line := fmt.Sprintf("%s%02d  %s", marker, i+1, item.Label)
		lines = append(lines, menuItemStyle(active).Width(width).Render(line))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderWorkspace(width int) string {
	shortcuts := workspaceShortcuts
	lines := []string{
		lipgloss.NewStyle().Foreground(accent).Bold(true).Render("GARFEX / MATERIALES MAESTROS"),
		"",
	}
	for i, shortcut := range shortcuts {
		marker := "  "
		if i == m.workspaceItem {
			marker = "› "
		}
		lines = append(lines, menuItemStyle(i == m.workspaceItem).Render(marker+shortcut.Label))
	}
	lines = append(lines, m.viewport.View())
	lines = append(lines, m.renderInteractionDock(width-2))
	return lipgloss.NewStyle().Width(width).Padding(1, 1).Foreground(primaryText).Background(surface).Render(strings.Join(lines, "\n"))
}

func (m Model) renderInteractionDock(width int) string {
	width = max(1, width)
	if m.interactionMode == interactionModeChat {
		if m.inputFocused {
			return lipgloss.NewStyle().Foreground(primaryText).Background(surface).Padding(0, 1).Width(width).Render("❯ " + m.input + "▌")
		}
		return lipgloss.NewStyle().Foreground(secondaryText).Render("❯ ¿Qué necesitas?")
	}
	hint := "↑↓ seleccionar · Enter confirmar · Esc cancelar"
	if request, ok := m.pending.(QuestionRequest); ok && request.SelectionMode == SelectionMultiple {
		hint += " · Espacio alternar"
	}
	return lipgloss.NewStyle().Foreground(secondaryText).Render(hint)
}

func (m Model) renderState(width int) string {
	label, detail, note := "Trabajando", "Buscando material...", "Esperá un momento"
	if m.screen == screenLoading {
		detail = "Buscando material..."
	}
	if m.screen == screenResult {
		label, detail, note = "Resultado", m.result, ""
	} else if m.screen == screenError {
		label, detail, note = "No se pudo completar la consulta", m.result, ""
	}

	lines := []string{lipgloss.NewStyle().Foreground(lipgloss.Color(stateAccent(m.screen))).Bold(true).Render(label)}
	if message := inputFromHistory(m.history); m.screen == screenLoading && message != "" {
		lines = append(lines, "> "+message)
	}
	lines = append(lines, detail)
	if note != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(secondaryText).Render(note))
	}
	return lipgloss.NewStyle().Width(width).Padding(0, 1).Foreground(primaryText).Background(surface).
		Render(strings.Join(lines, "\n"))
}

func stateAccent(current screen) string {
	if current == screenResult {
		return successHex
	}
	if current == screenError {
		return errorHex
	}
	return accentHex
}

func hint(key, description string) string {
	return lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(accent).Bold(true).Render(key),
		lipgloss.NewStyle().Foreground(secondaryText).Render(" "+description),
	)
}

func (m Model) renderFooter(width int) string {
	parts := []string{hint("ctrl+c", "salir")}
	if m.screen == screenHome {
		parts = []string{hint("flechas", "navegar"), hint("enter", "elegir"), hint("ctrl+c", "salir")}
	} else if m.screen == screenWorkspace {
		if m.interactionMode == interactionModeChoice || m.interactionMode == interactionModeConfirmation || m.interactionMode == interactionModeAction {
			parts = []string{hint("↑↓", "seleccionar"), hint("enter", "confirmar"), hint("esc", "cancelar")}
			if request, ok := m.pending.(QuestionRequest); ok && request.SelectionMode == SelectionMultiple {
				parts = append(parts, hint("espacio", "alternar"))
			}
		} else if m.inputFocused {
			parts = []string{hint("enter", "enviar"), hint("esc", "cancelar"), hint("ctrl+c", "salir")}
		} else {
			parts = []string{hint("flechas", "navegar"), hint("enter", "usar"), hint("esc", "volver"), hint("ctrl+c", "salir")}
		}
	} else if m.screen == screenLoading {
		parts = []string{hint("esc", "cancelar"), hint("ctrl+c", "salir")}
	} else if m.screen == screenResult {
		parts = []string{hint("enter", "reintentar"), hint("esc", "volver"), hint("ctrl+c", "salir")}
	} else if m.screen == screenError {
		parts = []string{hint("enter", "reintentar"), hint("esc", "volver"), hint("ctrl+c", "salir")}
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).
		Render(strings.Join(parts, "  "))
}
