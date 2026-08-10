package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	defaultWidth  = 72
	defaultHeight = 20
	maxCardWidth  = 68

	officialTagline  = "DISEÑO · CONSTRUCCIÓN · MANTENIMIENTO ELÉCTRICO"
	backgroundHex    = "#0B0D0E"
	surfaceHex       = "#17191B"
	primaryTextHex   = "#F2F0E9"
	secondaryTextHex = "#8B8B86"
	brandRedHex      = "#800000"
	accentHex        = "#FFD400"
	successHex       = "#4FC38A"
	errorHex         = "#FF6B6B"

	fullWordmark = `
███████    ██████   ████████  ████████  ████████  ███    ███
██        ██    ██  ██    ██  ██        ██         ███  ███
██  █████ ████████  ████████  ███████   ███████      █████
██    ███ ██    ██  ██  ███   ██        ██         ███  ███
█████████ ██    ██  ██    ██  ██        ████████  ███    ███
`
)

var (
	background    = lipgloss.Color(backgroundHex)
	surface       = lipgloss.Color(surfaceHex)
	primaryText   = lipgloss.Color(primaryTextHex)
	secondaryText = lipgloss.Color(secondaryTextHex)
	brandRed      = lipgloss.Color(brandRedHex)
	accent        = lipgloss.Color(accentHex)
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
	sections := []string{renderWordmark(width, full), renderTagline(width), renderDivider(width, full)}
	if m.screen == screenMenu {
		if full {
			sections = append(sections, lipgloss.NewStyle().Width(width).Foreground(secondaryText).Render("MENÚ PRINCIPAL"))
		}
		sections = append(sections, m.renderMenu(width))
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

func (m Model) renderState(width int) string {
	label, detail, note := "PROCESANDO", "Procesando "+m.items[m.cursor].Label+"...", "Espere un momento"
	if m.screen == screenResult {
		label, detail, note = "OPERACIÓN COMPLETADA", m.result, ""
	} else if m.screen == screenError {
		label, detail, note = "ERROR DE OPERACIÓN", m.result, ""
	}

	lines := []string{lipgloss.NewStyle().Foreground(lipgloss.Color(stateAccent(m.screen))).Bold(true).Render(label), detail}
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
	parts := []string{hint("q", "salir")}
	if m.screen == screenMenu {
		parts = []string{hint("j/k", "navegar"), hint("enter", "elegir"), hint("q", "salir")}
	} else if m.screen == screenResult {
		parts = []string{hint("b/esc", "volver"), hint("q", "salir")}
	} else if m.screen == screenError {
		parts = []string{hint("r", "reintentar"), hint("b/esc", "volver"), hint("q", "salir")}
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).
		Render(strings.Join(parts, "  "))
}
