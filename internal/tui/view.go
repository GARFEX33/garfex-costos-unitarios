package tui

import (
	"fmt"
	"strings"
	"unicode"

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
	switch m.screen {
	case screenHome:
		sections = append(sections, renderWordmark(width, full), renderTagline(width), renderDivider(width, full))
		if full {
			sections = append(sections, lipgloss.NewStyle().Width(width).Foreground(secondaryText).Render("HOME · ÁREAS DE GARFEX"))
		}
		sections = append(sections, m.renderMenu(width))
	case screenWorkspace:
		if m.heroActive {
			sections = append(sections, renderWordmark(width, full), renderTagline(width), renderDivider(width, full))
		}
		sections = append(sections, m.renderWorkspace(width))
	case screenManual:
		sections = append(sections, m.renderManual(width))
	default:
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
	var lines []string
	if !m.heroActive {
		header := "GARFEX / ASSISTANT"
		if slot, ok := m.workspaces[m.activeWorkspace]; ok {
			header = strings.ReplaceAll(slot.descriptor.Title, " / ", " › ") + " › " + m.workspaceModeLabel()
		}
		lines = append(lines,
			lipgloss.NewStyle().Foreground(accent).Bold(true).Render(header),
			"",
		)
	}
	lines = append(lines, m.viewport.View())
	lines = append(lines, m.renderInteractionDock(width-2))
	return lipgloss.NewStyle().Width(width).Padding(1, 1).Foreground(primaryText).Background(surface).Render(strings.Join(lines, "\n"))
}

func (m Model) workspaceModeLabel() string {
	if m.interactionMode == interactionModeMenu || m.interactionMode == interactionModePalette {
		return "Menú"
	}
	if m.interactionMode == interactionModeChat {
		return "Buscar"
	}
	if _, ok := m.pending.(ActionRequest); ok {
		return "Detalle"
	}
	if request, ok := m.pending.(QuestionRequest); ok && (request.Key == searchResultsKey || request.Key == catalogStatusMenuKey || request.Key == catalogKindMenuKey) {
		return "Lista"
	}
	return "Editar"
}

func (m Model) renderManual(width int) string {
	lines := []string{
		lipgloss.NewStyle().Foreground(accent).Bold(true).Render("GARFEX › Materiales › Buscar"),
		"",
	}
	if m.pending != nil {
		lines = append(lines, renderActiveSearchable(m.pending, m.searchQuery, m.choiceIndex))
	}
	return lipgloss.NewStyle().Width(width).Padding(1, 1).Foreground(primaryText).Background(surface).Render(strings.Join(lines, "\n"))
}

// hintPart is one (key, description) contextual-help fragment.
type hintPart struct{ Key, Description string }

// questionHelpParts builds the contextual help fragments for the pending
// question, shared by renderFooter (general footer) and renderInteractionDock
// (composer hint) so both stay in sync instead of maintaining two separate
// copies of the hint text. allowCustom is accepted for forward compatibility
// with the AllowCustom-editable composer (wired in a follow-up change); it is
// unused while that behavior is not yet implemented.
func questionHelpParts(pending InteractionMessage, allowCustom bool) []hintPart {
	if pending == nil {
		return nil
	}
	_ = allowCustom
	request, isQuestion := pending.(QuestionRequest)
	if isQuestion && request.SelectionMode == SelectionSearchable {
		return []hintPart{{"↑↓/j/k", "seleccionar"}, {"enter", "confirmar"}, {"esc", "cancelar"}}
	}
	parts := []hintPart{{"↑↓", "seleccionar"}, {"enter", "confirmar"}, {"esc", "cancelar"}}
	if isQuestion && request.SelectionMode == SelectionMultiple {
		parts = append(parts, hintPart{"espacio", "alternar"})
	}
	return parts
}

// capitalizeHintWord upper-cases the first rune of a hint fragment for the
// dock's contextual help line (non-cased runes such as arrows are left
// untouched).
func capitalizeHintWord(part string) string {
	runes := []rune(part)
	if len(runes) == 0 {
		return part
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func (m Model) renderInteractionDock(width int) string {
	width = max(1, width)
	if m.interactionMode == interactionModeChat {
		if m.activeWorkspace != "" {
			return lipgloss.NewStyle().Foreground(primaryText).Background(surface).Padding(0, 1).Width(width).Render("Buscar: " + m.input + "▌")
		}
		if m.inputFocused {
			return lipgloss.NewStyle().Foreground(primaryText).Background(surface).Padding(0, 1).Width(width).Render("❯ " + m.input + "▌")
		}
		return lipgloss.NewStyle().Foreground(secondaryText).Render("❯ Pregúntame o escribe / para ver acciones...")
	}
	if m.interactionMode == interactionModePalette {
		prompt := lipgloss.NewStyle().Foreground(primaryText).Background(surface).Padding(0, 1).Width(width).Render("❯ " + m.input + "▌")
		return strings.Join([]string{prompt, m.renderPalette()}, "\n")
	}
	if m.interactionMode == interactionModeMenu {
		return m.renderPalette()
	}
	parts := questionHelpParts(m.pending, false)
	flat := make([]string, len(parts))
	for i, part := range parts {
		flat[i] = capitalizeHintWord(part.Key + " " + part.Description)
	}
	hintLine := lipgloss.NewStyle().Foreground(secondaryText).Render(strings.Join(flat, " · "))
	if m.pendingAllowsCustom() {
		composer := lipgloss.NewStyle().Foreground(primaryText).Render("Editar: " + m.input + "▌")
		return strings.Join([]string{composer, hintLine}, "\n")
	}
	if m.activeWorkspace == "" {
		composer := lipgloss.NewStyle().Foreground(secondaryText).Render("❯ " + m.input)
		return strings.Join([]string{composer, hintLine}, "\n")
	}
	return hintLine
}

func (m Model) renderState(width int) string {
	label, detail, note := "Trabajando", "Buscando material...", "Esperá un momento"
	if m.screen == screenLoading {
		detail = "Buscando material..."
	}
	switch m.screen {
	case screenResult:
		label, detail, note = "Resultado", m.result, ""
	case screenError:
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
	switch m.screen {
	case screenHome:
		parts = []string{hint("flechas", "navegar"), hint("enter", "elegir"), hint("ctrl+c", "salir")}
	case screenWorkspace:
		if m.interactionMode == interactionModePalette || m.interactionMode == interactionModeMenu {
			parts = []string{hint("↑↓", "mover"), hint("enter", "seleccionar"), hint("esc", "volver"), hint("/", "acciones"), hint("?", "ayuda")}
		} else if m.interactionMode == interactionModeSearchable || m.interactionMode == interactionModeChoice || m.interactionMode == interactionModeConfirmation || m.interactionMode == interactionModeAction {
			parts = nil
			for _, part := range questionHelpParts(m.pending, false) {
				parts = append(parts, hint(part.Key, part.Description))
			}
		} else if m.inputFocused {
			parts = []string{hint("enter", "enviar"), hint("esc", "cancelar"), hint("ctrl+c", "salir")}
		} else {
			parts = []string{hint("flechas", "navegar"), hint("enter", "usar"), hint("esc", "volver"), hint("ctrl+c", "salir")}
		}
		if m.helpVisible {
			if slot := m.workspaces[m.activeWorkspace]; slot != nil && slot.descriptor.CreateLabel != "" && m.interactionMode == interactionModeMenu {
				parts = append(parts, hint("b", "buscar"), hint("+", "crear"))
			}
			for _, option := range m.pendingOptions() {
				switch option.ID {
				case catalogCreateNewOptionID:
					parts = append(parts, hint("+", "crear"))
				case editActionID, catalogRecordEditActionID:
					parts = append(parts, hint("e", "editar"))
				case duplicateActionID:
					parts = append(parts, hint("d", "duplicar"))
				case catalogRecordDeactivateActionID:
					parts = append(parts, hint("x", "desactivar"))
				case catalogRecordReactivateActionID:
					parts = append(parts, hint("r", "reactivar"))
				}
			}
		}
	case screenManual:
		parts = []string{hint("↑↓/j/k", "seleccionar"), hint("enter", "confirmar"), hint("esc", "volver")}
	case screenLoading:
		parts = []string{hint("esc", "cancelar"), hint("ctrl+c", "salir")}
	case screenResult:
		parts = []string{hint("enter", "reintentar"), hint("esc", "volver"), hint("ctrl+c", "salir")}
	case screenError:
		parts = []string{hint("enter", "reintentar"), hint("esc", "volver"), hint("ctrl+c", "salir")}
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).
		Render(strings.Join(parts, "  "))
}

func renderActiveSearchable(message InteractionMessage, query string, selected int) string {
	options := filterOptions(pendingOptionsFor(message), query)
	lines := []string{renderInteractionMessage(message), "Buscar: " + query, fmt.Sprintf("Coincidencias: %d", len(options))}
	for i, option := range options {
		label := "  " + option.Label
		if i == selected {
			label = "❯ " + option.Label
		}
		lines = append(lines, interactionOptionStyle(i == selected, false).Render(label))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderPalette() string {
	options := filterOptions(actionOptions(m.paletteActions), m.paletteQuery)
	var lines []string
	if m.paletteTitle != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(secondaryText).Render(m.paletteTitle))
	}
	for i, option := range options {
		label := "  " + option.Label
		if i == m.paletteIndex {
			label = "❯ " + option.Label
		}
		lines = append(lines, interactionOptionStyle(i == m.paletteIndex, false).Render(label))
	}
	return strings.Join(lines, "\n")
}
