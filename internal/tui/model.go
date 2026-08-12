package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	screenManual
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
type messageRole uint8

const (
	roleUser messageRole = iota
	roleGARFEX
)

type conversationMessage struct {
	role     messageRole
	message  InteractionMessage
	resolved *resolvedInteraction
	text     string
	kind     messageKind
}

type resolvedInteraction struct {
	prompt    string
	selection string
}
type messageKind uint8

const (
	kindText messageKind = iota
	kindProgress
	kindError
	kindResult
	kindQuestion
)

type interactionMode uint8

const (
	interactionModeChat interactionMode = iota
	interactionModeChoice
	interactionModeSearchable
	interactionModeConfirmation
	interactionModeAction
	interactionModePalette
)

type Model struct {
	items                  []Item
	cursor, workspaceItem  int
	input                  string
	history                []conversationMessage
	promptHistory          []string
	promptHistoryCursor    int
	viewport               viewport.Model
	inputFocused           bool
	screen, workspaceState screen
	width, height          int
	result                 string
	engine                 *InteractionEngine
	interactionMode        interactionMode
	pending                InteractionMessage
	choiceIndex            int
	choicePrompt           string
	choiceOptions          []string
	searchQuery            string
	workspacePending       bool
	paletteQuery           string
	paletteIndex           int
	paletteActions         []assistantAction
	paletteTitle           string
	paletteFlat            bool
	heroActive             bool
	manualReturnInput      string
	manualReturnOffset     int
	manualReturnAtBottom   bool
}

func New(handlers Handlers) Model {
	m := Model{items: []Item{{Label: "Materiales Maestros"}, {Label: "Versión", Handler: handlers.Version}, {Label: "Verificar configuración", Handler: handlers.Config}, {Label: "Estado de GARFEX", Handler: handlers.Status}, {Label: "Salir", Quit: true}}, engine: NewInteractionEngine(NewFakeAgent()), screen: screenWorkspace, viewport: viewport.New(viewport.WithWidth(defaultWidth-4), viewport.WithHeight(workspaceViewportHeight(defaultHeight, interactionModeChat))), inputFocused: true, promptHistoryCursor: -1, heroActive: true}
	m.viewport.SoftWrap, m.viewport.FillHeight = true, true
	m.refreshViewport()
	return m
}

func NewWithAgent(handlers Handlers, agent InteractionAgent) Model {
	m := New(handlers)
	m.engine = NewInteractionEngine(agent)
	return m
}

func (Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeViewport()
		if m.width < minWidth || m.height < minHeight {
			m.screen = screenMinSize
		} else if m.screen == screenMinSize {
			m.screen = screenMenu
		}
		return m, nil
	case resultMsg:
		m.result = sanitize(msg.text)
		if m.screen == screenWorkspace {
			if msg.err != nil {
				m.appendGARFEX(ErrorMessage{Text: naturalError(msg.err)})
				m.workspaceState = screenError
			} else {
				m.appendGARFEX(StructuredResult{Title: "Resultado", Fields: []Field{{Label: "Detalle", Value: m.result}}})
				m.workspaceState = screenResult
			}
			return m, nil
		}
		if msg.err != nil {
			m.result, m.screen = naturalError(msg.err), screenError
		} else {
			m.screen = screenResult
		}
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.screen == screenManual {
			if key == "esc" {
				m.leaveManual()
				return m, nil
			}
			m.handlePendingKey(msg)
			return m, nil
		}
		if m.screen == screenWorkspace {
			if m.interactionMode == interactionModePalette {
				m.handlePaletteKey(msg)
				return m, nil
			}
			if m.interactionMode != interactionModeChat {
				if m.scrollHistory(msg) {
					return m, nil
				}
				m.handlePendingKey(msg)
				return m, nil
			}
			if m.scrollChatHistory(msg) {
				return m, nil
			}
			if m.inputFocused {
				if key == "up" || key == "down" {
					m.navigatePromptHistory(key)
					return m, nil
				}
				switch key {
				case "esc":
					m.inputFocused = false
				case "enter":
					input := strings.TrimSpace(m.input)
					if input == "" {
						return m, nil
					}
					m.heroActive = false
					m.input = ""
					m.addPromptHistory(input)
					m.appendUser(TextMessage{Text: input})
					m.inputFocused = false
					m.respond(InteractionInput{Kind: InputText, Value: input})
				case "backspace":
					m.input = trimLastRune(m.input)
					m.promptHistoryCursor = -1
				default:
					if msg.Text != "" && !strings.ContainsAny(msg.Text, "\x00\x1b") {
						m.heroActive = false
						m.input += msg.Text
						m.promptHistoryCursor = -1
						if strings.HasPrefix(m.input, "/") {
							m.openPalette(strings.TrimPrefix(m.input, "/"))
						}
					}
				}
				return m, nil
			}
		}
		if (m.screen == screenLoading || m.screen == screenResult || m.screen == screenError) && key == "esc" {
			m.screen, m.inputFocused = screenHome, false
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
					m.screen, m.workspaceItem, m.inputFocused = screenWorkspace, 0, false
					return m, nil
				}
				m.screen = screenLoading
				if item.Handler == nil {
					return m, func() tea.Msg { return resultMsg{text: "Área disponible para una próxima entrega."} }
				}
				return m, item.Handler()
			}
		} else if m.screen == screenWorkspace {
			if !m.inputFocused && key == "enter" {
				m.inputFocused = true
			}
		} else if (m.screen == screenResult || m.screen == screenError) && key == "enter" {
			m.screen = screenLoading
			return m, simulateInput(m.input)
		}
	}
	return m, nil
}

func (m *Model) respond(input InteractionInput) {
	wasAtBottom := m.viewport.AtBottom()
	response := m.engine.Respond(context.Background(), input)
	for _, message := range response.Messages {
		if isActivePendingMessage(message, response.Pending) {
			continue
		}
		m.appendGARFEX(message)
	}
	m.pending = response.Pending
	m.workspacePending = response.Pending != nil
	m.interactionMode = modeFor(response.Pending)
	m.choiceIndex = 0
	m.syncChoiceFields()
	m.refreshViewport()
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

func (m *Model) openPalette(query string) {
	m.heroActive = false
	m.paletteFlat = query != ""
	if query == "" {
		m.paletteActions = assistantActions
	} else {
		m.paletteActions = flattenLeafActions(assistantActions)
	}
	m.paletteTitle = ""
	m.paletteQuery = query
	m.paletteIndex = 0
	m.interactionMode = interactionModePalette
	m.input = "/" + query
	m.refreshViewport()
}

func (m *Model) handlePaletteKey(msg tea.KeyPressMsg) {
	key := msg.String()
	options := filterOptions(actionOptions(m.paletteActions), m.paletteQuery)
	switch key {
	case "esc":
		m.paletteQuery, m.paletteIndex, m.paletteActions = "", 0, nil
		m.paletteFlat = false
		m.interactionMode, m.inputFocused = interactionModeChat, true
		m.refreshViewport()
	case "up", "k":
		if m.paletteIndex > 0 {
			m.paletteIndex--
		}
	case "down", "j":
		if m.paletteIndex < len(options)-1 {
			m.paletteIndex++
		}
	case "backspace":
		m.paletteQuery = trimLastRune(m.paletteQuery)
		m.input = "/" + m.paletteQuery
		m.paletteIndex = 0
		if m.paletteQuery == "" {
			if m.paletteFlat {
				m.paletteFlat = false
				m.paletteActions = assistantActions
			}
		} else if !m.paletteFlat {
			m.paletteFlat = true
			m.paletteActions = flattenLeafActions(assistantActions)
		}
	case "enter":
		if len(options) == 0 {
			return
		}
		selected := options[m.paletteIndex]
		for _, action := range m.paletteActions {
			if action.id != selected.ID {
				continue
			}
			if len(action.children) > 0 {
				m.paletteActions, m.paletteTitle, m.paletteQuery, m.paletteIndex = action.children, action.label, "", 0
				m.paletteFlat = false
				m.refreshViewport()
				return
			}
			if action.id == "material-search" {
				m.openManualSearch()
			}
			return
		}
	default:
		if msg.Text != "" && !strings.ContainsAny(msg.Text, "\x00\x1b") {
			m.paletteQuery += msg.Text
			m.input = "/" + m.paletteQuery
			m.paletteIndex = 0
			if !m.paletteFlat {
				m.paletteFlat = true
				m.paletteActions = flattenLeafActions(assistantActions)
			}
		}
	}
	m.refreshViewport()
}

func (m *Model) openManualSearch() {
	m.heroActive = false
	m.manualReturnInput = m.input
	m.manualReturnOffset = m.viewport.YOffset()
	m.manualReturnAtBottom = m.viewport.AtBottom()
	m.paletteQuery, m.paletteIndex, m.paletteActions = "", 0, nil
	m.paletteTitle = ""
	m.input = ""
	m.inputFocused = false
	m.screen = screenManual
	m.pending = QuestionRequest{ID: "material-search", Key: "material-search", Prompt: "Buscar material", SelectionMode: SelectionSearchable, Options: manualMaterialOptions()}
	m.interactionMode = interactionModeSearchable
	m.choiceIndex, m.searchQuery = 0, ""
	m.syncChoiceFields()
	m.refreshViewport()
}

func (m *Model) leaveManual() {
	returnInput, returnOffset, returnAtBottom := m.manualReturnInput, m.manualReturnOffset, m.manualReturnAtBottom
	m.manualReturnInput, m.manualReturnOffset, m.manualReturnAtBottom = "", 0, false
	m.pending, m.searchQuery = nil, ""
	m.interactionMode, m.screen, m.inputFocused, m.input = interactionModeChat, screenWorkspace, true, returnInput
	m.syncChoiceFields()
	m.refreshViewport()
	if returnAtBottom {
		m.viewport.GotoBottom()
	} else {
		m.viewport.SetYOffset(returnOffset)
	}
}

func (m *Model) handleShortcut(id string) {
	switch id {
	case "new_search":
		m.resetPending()
		m.inputFocused = true
	case "create_material":
		m.resetPending()
		m.appendGARFEX(TextMessage{Text: "Podés describir el material que querés crear."})
		m.inputFocused = true
	case "view_catalog":
		m.appendGARFEX(TextMessage{Text: "El catálogo está listo para consultar familias y unidades."})
	case "view_history":
		m.viewport.GotoBottom()
	}
}

func (m *Model) resetPending() {
	m.pending = nil
	m.workspacePending = false
	m.interactionMode = interactionModeChat
	m.choiceIndex = 0
	m.searchQuery = ""
	m.syncChoiceFields()
	m.refreshViewport()
}

func isActivePendingMessage(message, pending InteractionMessage) bool {
	if message == nil || pending == nil || pendingQuestion(message) == "" || pendingQuestion(message) != pendingQuestion(pending) {
		return false
	}
	switch message.(type) {
	case QuestionRequest:
		_, ok := pending.(QuestionRequest)
		return ok
	case ConfirmationRequest:
		_, ok := pending.(ConfirmationRequest)
		return ok
	case ActionRequest:
		_, ok := pending.(ActionRequest)
		return ok
	default:
		return false
	}
}

func modeFor(pending InteractionMessage) interactionMode {
	switch request := pending.(type) {
	case QuestionRequest:
		if request.SelectionMode == SelectionSearchable {
			return interactionModeSearchable
		}
		return interactionModeChoice
	case ConfirmationRequest:
		return interactionModeConfirmation
	case ActionRequest:
		return interactionModeAction
	default:
		return interactionModeChat
	}
}

func (m *Model) handlePendingKey(msg tea.KeyPressMsg) {
	if m.interactionMode == interactionModeSearchable {
		m.handleSearchableKey(msg)
		return
	}
	key := msg.String()
	options := m.pendingOptions()
	switch key {
	case "up":
		if m.choiceIndex > 0 {
			m.choiceIndex--
			m.refreshViewport()
		}
	case "down":
		if m.choiceIndex < len(options)-1 {
			m.choiceIndex++
			m.refreshViewport()
		}
	case "esc":
		m.respond(InteractionInput{Kind: InputCancel})
	case "enter":
		if len(options) > 0 {
			selected := options[m.choiceIndex]
			m.appendResolvedInteraction(pendingQuestion(m.pending), selected.Label)
			input := InteractionInput{Kind: inputKindFor(m.pending), Key: pendingKey(m.pending), Value: selected.Value, ActionID: selected.ID, Target: selected.Target}
			if selected.Target == ActionTargetLocal {
				m.handleLocalAction(input)
				return
			}
			m.respond(input)
		}
	}
}

func (m *Model) handleSearchableKey(msg tea.KeyPressMsg) {
	key := msg.String()
	if key == "esc" {
		m.searchQuery = ""
		m.respond(InteractionInput{Kind: InputCancel})
		return
	}
	if key == "up" || key == "k" {
		if m.choiceIndex > 0 {
			m.choiceIndex--
			m.refreshViewport()
		}
		return
	}
	if key == "down" || key == "j" {
		options := m.pendingOptions()
		if m.choiceIndex < len(options)-1 {
			m.choiceIndex++
			m.refreshViewport()
		}
		return
	}
	if key == "enter" {
		options := m.pendingOptions()
		if len(options) == 0 {
			return
		}
		selected := options[m.choiceIndex]
		m.appendResolvedInteraction(pendingQuestion(m.pending), selected.Label)
		m.searchQuery = ""
		if m.screen == screenManual {
			m.appendGARFEX(StructuredResult{Title: "Material seleccionado", Fields: []Field{{Label: "Material", Value: selected.Label}}})
			m.leaveManual()
			return
		}
		m.respond(InteractionInput{Kind: InputSelection, Key: pendingKey(m.pending), Value: selected.Value})
		return
	}
	if key == "backspace" {
		m.searchQuery = trimLastRune(m.searchQuery)
		m.syncChoiceFields()
		m.refreshViewport()
		return
	}
	if msg.Text != "" && !strings.ContainsAny(msg.Text, "\x00\x1b") {
		m.searchQuery += msg.Text
		m.syncChoiceFields()
		m.refreshViewport()
	}
}

func (m *Model) handleLocalAction(input InteractionInput) {
	switch input.ActionID {
	case "new_search":
		m.resetPending()
		m.inputFocused = true
	case "back_material":
		m.resetPending()
	case "view_history":
		m.viewport.GotoBottom()
	}
}

func inputKindFor(pending InteractionMessage) InputKind {
	if _, ok := pending.(ActionRequest); ok {
		return InputAction
	}
	return InputSelection
}

func (m Model) pendingOptions() []Option {
	if m.interactionMode == interactionModeSearchable {
		return filterOptions(pendingOptionsFor(m.pending), m.searchQuery)
	}
	return pendingOptionsFor(m.pending)
}

func filterOptions(options []Option, query string) []Option {
	tokens := strings.Fields(strings.ToLower(query))
	if len(tokens) == 0 {
		return append([]Option(nil), options...)
	}
	filtered := make([]Option, 0, len(options))
	for _, option := range options {
		label := strings.ToLower(option.Label)
		matches := true
		for _, token := range tokens {
			if !strings.Contains(label, token) {
				matches = false
				break
			}
		}
		if matches {
			filtered = append(filtered, option)
		}
	}
	return filtered
}

func pendingOptionsFor(message InteractionMessage) []Option {
	switch request := message.(type) {
	case QuestionRequest:
		return request.Options
	case ConfirmationRequest:
		return []Option{{Label: request.ConfirmLabel, Value: "yes"}, {Label: request.CancelLabel, Value: "no"}}
	case ActionRequest:
		options := make([]Option, len(request.Actions))
		for i, action := range request.Actions {
			options[i] = Option{ID: action.ID, Label: action.Label, Value: action.Value, Target: action.Target}
		}
		return options
	}
	return nil
}

func (m *Model) appendUser(message InteractionMessage) {
	m.appendMessage(conversationMessage{role: roleUser, message: message, text: messageText(message), kind: kindText})
}
func (m *Model) appendGARFEX(message InteractionMessage) {
	kind := kindText
	switch message.(type) {
	case ErrorMessage:
		kind = kindError
	case StructuredResult:
		kind = kindResult
	case QuestionRequest, ConfirmationRequest, ActionRequest:
		kind = kindQuestion
	}
	m.appendMessage(conversationMessage{role: roleGARFEX, message: message, text: messageText(message), kind: kind})
}

func (m *Model) appendResolvedInteraction(prompt, selection string) {
	wasAtBottom := m.viewport.AtBottom()
	m.history = append(m.history, conversationMessage{
		role:     roleGARFEX,
		resolved: &resolvedInteraction{prompt: prompt, selection: selection},
		kind:     kindQuestion,
	})
	m.refreshViewport()
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

func (m *Model) syncChoiceFields() {
	m.choicePrompt = pendingQuestion(m.pending)
	options := m.pendingOptions()
	if len(options) == 0 {
		m.choiceIndex = 0
	} else if m.choiceIndex >= len(options) {
		m.choiceIndex = len(options) - 1
	}
	m.choiceOptions = make([]string, len(options))
	for i, option := range options {
		m.choiceOptions[i] = option.Label
	}
}

func (m *Model) setPendingQuestion(prompt string, options []string) {
	request := QuestionRequest{ID: "question", Key: "question", Prompt: prompt, Question: prompt, SelectionMode: SelectionSingle}
	for _, option := range options {
		request.Options = append(request.Options, Option{Label: option, Value: option})
	}
	m.pending = request
	m.interactionMode = interactionModeChoice
	m.searchQuery = ""
	m.choiceIndex = 0
	m.syncChoiceFields()
	m.resizeViewport()
}

func (m *Model) resizeViewport() {
	wasAtBottom := m.viewport.AtBottom()
	width, height := m.width, m.height
	if width == 0 {
		width = defaultWidth
	}
	if height == 0 {
		height = defaultHeight
	}
	viewportWidth := max(1, width-4)
	m.viewport.SetWidth(viewportWidth)
	maxHeight := max(1, height-fixedWorkspaceHeight-interactionDockHeight)
	if m.screen == screenWorkspace {
		maxHeight = max(1, height-fixedWorkspaceHeight-m.interactionDockLines(viewportWidth-2))
	}
	m.viewport.SetHeight(maxHeight)
	m.refreshViewport()
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

const (
	workspaceHeaderHeight  = 6
	workspacePaddingHeight = 2
	workspaceFooterHeight  = 1
)

const (
	fixedWorkspaceHeight  = workspaceHeaderHeight + workspacePaddingHeight + workspaceFooterHeight
	interactionDockHeight = 1
)

func workspaceViewportHeight(height int, mode interactionMode) int {
	return max(1, height-fixedWorkspaceHeight-interactionDockHeight)
}

func (m *Model) refreshViewport() {
	lines := make([]string, 0, len(m.history)*3)
	for _, message := range m.history {
		if message.resolved != nil {
			lines = append(lines, renderResolvedInteraction(*message.resolved), "")
			continue
		}
		if message.role == roleUser {
			lines = append(lines, "❯ "+messageText(message.message), "")
			continue
		}
		lines = append(lines, renderInteractionMessage(message.message), "")
	}
	if m.pending != nil {
		if m.interactionMode == interactionModeSearchable {
			lines = append(lines, renderActiveSearchable(m.pending, m.searchQuery, m.choiceIndex), "")
		} else {
			lines = append(lines, renderActiveInteraction(m.pending, m.choiceIndex), "")
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "Todavía no hay mensajes. Iniciá una búsqueda para comenzar.")
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
	maxHeight := max(1, m.height-fixedWorkspaceHeight-m.interactionDockLines(m.viewport.Width()-2))
	if m.height == 0 {
		maxHeight = max(1, defaultHeight-fixedWorkspaceHeight-m.interactionDockLines(m.viewport.Width()-2))
	}
	contentHeight := max(1, strings.Count(m.viewport.GetContent(), "\n")+1)
	m.viewport.SetHeight(min(maxHeight, contentHeight))
}

func (m Model) interactionDockLines(width int) int {
	return max(1, lipgloss.Height(m.renderInteractionDock(width)))
}

func renderResolvedInteraction(interaction resolvedInteraction) string {
	return interaction.prompt + "\n✓ " + interaction.selection
}

func renderActiveInteraction(message InteractionMessage, selected int) string {
	lines := []string{renderInteractionMessage(message)}
	options := pendingOptionsFor(message)
	for i, option := range options {
		active := i == selected
		label := "  " + option.Label
		if active {
			label = "❯ " + option.Label
		}
		lines = append(lines, interactionOptionStyle(active).Render(label))
	}
	return strings.Join(lines, "\n")
}

func messageText(message InteractionMessage) string {
	if message == nil {
		return ""
	}
	switch value := message.(type) {
	case TextMessage:
		return value.Text
	case ErrorMessage:
		return value.Text
	default:
		return renderInteractionMessage(message)
	}
}
func renderInteractionMessage(message InteractionMessage) string {
	switch value := message.(type) {
	case TextMessage:
		return value.Text
	case ErrorMessage:
		return value.Text
	case QuestionRequest:
		return questionPrompt(value)
	case ConfirmationRequest:
		return value.Question
	case ActionRequest:
		return value.Question
	case StructuredResult:
		var b strings.Builder
		b.WriteString(value.Title)
		if value.Subtitle != "" {
			b.WriteString("\n" + value.Subtitle)
		}
		for _, field := range value.Fields {
			fmt.Fprintf(&b, "\n%s: %s", field.Label, field.Value)
		}
		for _, section := range value.Sections {
			if section.Title != "" {
				b.WriteString("\n" + section.Title)
			}
			for _, field := range section.Fields {
				fmt.Fprintf(&b, "\n%s: %s", field.Label, field.Value)
			}
		}
		return b.String()
	}
	return ""
}
func pendingQuestion(message InteractionMessage) string {
	switch value := message.(type) {
	case QuestionRequest:
		return questionPrompt(value)
	case ConfirmationRequest:
		return value.Question
	case ActionRequest:
		return value.Question
	}
	return ""
}

func questionPrompt(request QuestionRequest) string {
	if request.Prompt != "" {
		return request.Prompt
	}
	return request.Question
}

func pendingKey(message InteractionMessage) string {
	switch value := message.(type) {
	case QuestionRequest:
		return value.Key
	case ConfirmationRequest:
		return value.Key
	case ActionRequest:
		return value.Key
	default:
		return ""
	}
}

func (m *Model) appendMessage(message conversationMessage) {
	if message.message == nil && message.text != "" {
		message.message = TextMessage{Text: message.text}
	}
	if message.text == "" {
		message.text = messageText(message.message)
	}
	wasAtBottom := len(m.history) == 0 || m.viewport.AtBottom()
	m.history = append(m.history, message)
	m.refreshViewport()
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}
func (m *Model) scrollHistory(msg tea.KeyPressMsg) bool {
	switch msg.Code {
	case tea.KeyPgUp:
		m.viewport.PageUp()
	case tea.KeyPgDown:
		m.viewport.PageDown()
	case tea.KeyHome:
		m.viewport.GotoTop()
	case tea.KeyEnd:
		m.viewport.GotoBottom()
	default:
		return false
	}
	return true
}

func (m *Model) scrollChatHistory(msg tea.KeyPressMsg) bool {
	return m.scrollHistory(msg)
}

func (m *Model) addPromptHistory(value string) {
	if len(m.promptHistory) == 0 || m.promptHistory[len(m.promptHistory)-1] != value {
		m.promptHistory = append(m.promptHistory, value)
	}
	m.promptHistoryCursor = -1
}

func (m *Model) navigatePromptHistory(key string) bool {
	if len(m.promptHistory) == 0 {
		return true
	}
	if key == "up" {
		if m.promptHistoryCursor < 0 {
			m.promptHistoryCursor = len(m.promptHistory)
		}
		if m.promptHistoryCursor > 0 {
			m.promptHistoryCursor--
			m.input = m.promptHistory[m.promptHistoryCursor]
		}
		return true
	}
	if key == "down" {
		if m.promptHistoryCursor < 0 {
			return true
		}
		if m.promptHistoryCursor < len(m.promptHistory)-1 {
			m.promptHistoryCursor++
			m.input = m.promptHistory[m.promptHistoryCursor]
		} else {
			m.promptHistoryCursor = -1
			m.input = ""
		}
		return true
	}
	return false
}
func inputFromHistory(history []conversationMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].role == roleUser {
			if history[i].message != nil {
				return messageText(history[i].message)
			}
			return history[i].text
		}
	}
	return ""
}
func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}
func naturalError(err error) string {
	if err == nil {
		return "No pude completar esa consulta."
	}
	return sanitize(err.Error())
}
func simulateInput(input string) tea.Cmd {
	return func() tea.Msg {
		if strings.Contains(strings.ToLower(input), "error") {
			return resultMsg{err: errors.New("No pude completar esa consulta.")}
		}
		return resultMsg{text: "Encontré una coincidencia para: " + sanitize(strings.TrimSpace(input))}
	}
}
func simulateShortcut(item int) tea.Cmd {
	return func() tea.Msg { return resultMsg{text: shortcutResponse(item)} }
}

func shortcutResponse(item int) string {
	responses := []string{"", "Podés describir el material y sus atributos para preparar una nueva ficha.", "El catálogo está listo para consultar familias y unidades.", "Las consultas recientes aparecen en esta conversación."}
	if item < 0 || item >= len(responses) {
		return ""
	}
	return responses[item]
}

func (m Model) View() tea.View {
	if m.screen == screenMinSize {
		return tea.NewView("La terminal debe tener al menos 40x10.")
	}
	return tea.NewView(m.render())
}
