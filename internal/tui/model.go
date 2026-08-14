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
type Handlers struct{ Version, Config, Status, Resources Handler }
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
	cancelled bool
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
	choiceSelected         []bool
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
	// assistantSaved snapshots the Assistant's own conversation while a
	// registered workspace is active; nil whenever the Assistant is active.
	assistantSaved *workspaceState
	// activeWorkspace, workspaces and workspaceOrder are the N-workspace
	// registry (recursos-maestro design §6): every specialized-workspace
	// literal site (palette actions, header, Esc-to-leave, palette confirm)
	// reads this registry instead of a hardcoded catalog name.
	//
	// activeWorkspace identifies which registered workspace slot (if any) is
	// currently active in place of the Assistant's own chat; "" means the
	// Assistant itself.
	activeWorkspace string
	// workspaces is the registry itself, keyed by WorkspaceDescriptor.Slug.
	workspaces map[string]*workspaceSlot
	// workspaceOrder lists every registered slug in registration order, for
	// callers that need deterministic iteration (e.g. a future palette
	// builder) — the map above is not itself ordered.
	workspaceOrder []string
}

// WorkspaceDescriptor is one registered specialized workspace's static shape
// (recursos-maestro design §6): the unfiltered "/recursos" workspace, or one
// class-scoped workspace like "/materiales" (ClassCode == "MATERIAL").
// BuildWorkspaceDescriptors (a later PR) is what turns a
// domain.ResourceCatalog into a []WorkspaceDescriptor.
type WorkspaceDescriptor struct {
	// Slug is both the registry key (Model.workspaces) and the palette
	// action id that enters this workspace (e.g. "recursos", "materiales").
	Slug string
	// Title is the workspace header rendered in place of "GARFEX / ASSISTANT"
	// while this workspace is active (e.g. "GARFEX / MATERIALES").
	Title string
	// CreateLabel is this workspace's scoped "/" palette leaf label (e.g.
	// "Crear material").
	CreateLabel string
	// ClassCode narrows this workspace to one domain.ResourceClass.Code; ""
	// means the unfiltered "/recursos" workspace.
	ClassCode string
	// Agent is this workspace's own InteractionAgent, set once at
	// construction (see NewWithWorkspaces) and never touched again.
	Agent InteractionAgent
}

// workspaceSlot is one live registry entry: the static descriptor it was
// built from, plus the conversation state it persists across visits (nil
// until the first entry, then always non-nil so re-entering resumes where
// the user left off).
type workspaceSlot struct {
	descriptor WorkspaceDescriptor
	agent      InteractionAgent
	saved      *workspaceState
}

// workspaceState snapshots everything that makes up one independent
// conversation's state (the Assistant's own chat, or a specialized catalog
// workspace's own chat), so switching between them never mixes history,
// pending interactions, or drafts. Screen-navigation-global fields (screen,
// workspaceItem, cursor, palette fields, manual-search-return fields) are
// deliberately excluded: they belong to TUI navigation, not to a
// conversation.
type workspaceState struct {
	engine           *InteractionEngine
	history          []conversationMessage
	input            string
	interactionMode  interactionMode
	pending          InteractionMessage
	choiceIndex      int
	choicePrompt     string
	choiceOptions    []string
	searchQuery      string
	choiceSelected   []bool
	workspacePending bool
	inputFocused     bool
	heroActive       bool
	// viewportOffset/viewportAtBottom preserve scroll position across a
	// workspace switch, restored after refreshViewport (not inside
	// applyWorkspace) so SetContent cannot clobber it first — the same
	// ordering leaveManual already relies on.
	viewportOffset   int
	viewportAtBottom bool
}

// snapshotWorkspace captures the current top-level conversation fields (and
// scroll position) into a workspaceState.
func (m *Model) snapshotWorkspace() workspaceState {
	return workspaceState{
		engine:           m.engine,
		history:          m.history,
		input:            m.input,
		interactionMode:  m.interactionMode,
		pending:          m.pending,
		choiceIndex:      m.choiceIndex,
		choicePrompt:     m.choicePrompt,
		choiceOptions:    m.choiceOptions,
		searchQuery:      m.searchQuery,
		choiceSelected:   m.choiceSelected,
		workspacePending: m.workspacePending,
		inputFocused:     m.inputFocused,
		heroActive:       m.heroActive,
		viewportOffset:   m.viewport.YOffset(),
		viewportAtBottom: m.viewport.AtBottom(),
	}
}

// applyWorkspace restores a workspaceState onto the top-level conversation
// fields. It deliberately does not touch scroll position — callers restore
// that via restoreViewportPosition after refreshViewport, matching the
// existing leaveManual ordering.
func (m *Model) applyWorkspace(s workspaceState) {
	m.engine = s.engine
	m.history = s.history
	m.input = s.input
	m.interactionMode = s.interactionMode
	m.pending = s.pending
	m.choiceIndex = s.choiceIndex
	m.choicePrompt = s.choicePrompt
	m.choiceOptions = s.choiceOptions
	m.searchQuery = s.searchQuery
	m.choiceSelected = s.choiceSelected
	m.workspacePending = s.workspacePending
	m.inputFocused = s.inputFocused
	m.heroActive = s.heroActive
}

// restoreViewportPosition restores the scroll position captured by
// snapshotWorkspace. Callers must call it after refreshViewport, since
// SetContent can otherwise clamp/reset the offset first.
func (m *Model) restoreViewportPosition(s workspaceState) {
	if s.viewportAtBottom {
		m.viewport.GotoBottom()
		return
	}
	m.viewport.SetYOffset(s.viewportOffset)
}

// enterWorkspace switches into the registered workspace identified by slug
// (WorkspaceDescriptor.Slug), saving the Assistant's current conversation
// first, using the shared snapshotWorkspace/applyWorkspace/assistantSaved
// machinery. A returning visit resumes exactly where it left off (slot.saved
// != nil); a
// first visit starts fresh with its own engine bound to the slot's Agent. It
// returns false and leaves every field untouched when slug is not a
// registered workspace.
func (m *Model) enterWorkspace(slug string) bool {
	slot, ok := m.workspaces[slug]
	if !ok {
		return false
	}
	saved := m.snapshotWorkspace()
	m.assistantSaved = &saved
	if slot.saved != nil {
		m.applyWorkspace(*slot.saved)
		m.activeWorkspace = slug
		m.refreshViewport()
		m.restoreViewportPosition(*slot.saved)
		return true
	}
	m.engine = NewInteractionEngine(slot.agent)
	m.history = nil
	m.input = ""
	m.interactionMode = interactionModeChat
	m.pending = nil
	m.choiceIndex = 0
	m.choicePrompt = ""
	m.choiceOptions = nil
	m.searchQuery = ""
	m.choiceSelected = nil
	m.workspacePending = false
	m.inputFocused = true
	m.heroActive = false
	if g, ok := slot.agent.(greeter); ok {
		m.appendGARFEX(g.Greeting())
	}
	m.activeWorkspace = slug
	m.refreshViewport()
	return true
}

// leaveActiveWorkspace saves the active registered workspace's own
// conversation (so re-entering it later resumes where the user left off) and
// restores the Assistant's conversation exactly as it was before entering.
// It is a no-op when no registered workspace is active.
func (m *Model) leaveActiveWorkspace() {
	if m.activeWorkspace == "" {
		return
	}
	slot := m.workspaces[m.activeWorkspace]
	saved := m.snapshotWorkspace()
	slot.saved = &saved
	restore := *m.assistantSaved
	m.applyWorkspace(restore)
	m.assistantSaved = nil
	m.activeWorkspace = ""
	m.refreshViewport()
	m.restoreViewportPosition(restore)
}

func New(handlers Handlers) Model {
	m := Model{items: []Item{{Label: "Materiales Maestros"}, {Label: "Versión", Handler: handlers.Version}, {Label: "Verificar configuración", Handler: handlers.Config}, {Label: "Estado de GARFEX", Handler: handlers.Status}, {Label: "Salir", Quit: true}}, engine: NewInteractionEngine(NewFakeAgent()), screen: screenWorkspace, viewport: viewport.New(viewport.WithWidth(defaultWidth-4), viewport.WithHeight(workspaceViewportHeight(defaultHeight, interactionModeChat))), inputFocused: true, promptHistoryCursor: -1, heroActive: true}
	m.viewport.SoftWrap, m.viewport.FillHeight = true, true
	m.refreshViewport()
	return m
}

// greeter is an optional capability an InteractionAgent may implement to
// seed an initial status message into the workspace before any user input.
type greeter interface {
	Greeting() InteractionMessage
}

func NewWithAgent(handlers Handlers, agent InteractionAgent) Model {
	m := New(handlers)
	m.engine = NewInteractionEngine(agent)
	if g, ok := agent.(greeter); ok {
		m.appendGARFEX(g.Greeting())
		m.refreshViewport()
	}
	return m
}

// NewWithWorkspaces wires the Assistant's own agent plus every registered
// workspace descriptor (recursos-maestro design §6). Every existing test call
// site that only needs the Assistant keeps using New/NewWithAgent unmodified,
// leaving workspaces nil (enterWorkspace already has a defined "unknown slug"
// false-return fallback, mirroring NewInteractionEngine's nil-agent
// fallback).
func NewWithWorkspaces(handlers Handlers, assistant InteractionAgent, descriptors []WorkspaceDescriptor) Model {
	m := NewWithAgent(handlers, assistant)
	m.workspaces = make(map[string]*workspaceSlot, len(descriptors))
	m.workspaceOrder = make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		m.workspaces[descriptor.Slug] = &workspaceSlot{descriptor: descriptor, agent: descriptor.Agent}
		m.workspaceOrder = append(m.workspaceOrder, descriptor.Slug)
	}
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
					// A single Esc from inside a registered workspace's plain
					// chat state always returns to the Assistant, predictably
					// (see leaveActiveWorkspace). Esc still just unfocuses the
					// composer when the Assistant itself is active; that
					// exact existing behavior is unchanged here.
					if m.activeWorkspace != "" {
						m.leaveActiveWorkspace()
					} else {
						m.inputFocused = false
					}
				case "enter":
					input := strings.TrimSpace(m.input)
					if input == "" {
						return m, nil
					}
					m.heroActive = false
					m.input = ""
					m.addPromptHistory(input)
					m.appendUser(TextMessage{Text: input})
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
	// Keep the composer ready to type immediately when a response has no
	// follow-up question (e.g. a plain search result): the user should not
	// have to press Enter a second time just to refocus the input before
	// typing their next message. When a question IS pending, focus moves to
	// answering it instead (handlePendingKey), matching existing behavior.
	m.inputFocused = m.interactionMode == interactionModeChat
	m.choiceIndex = 0
	m.choiceSelected = nil
	m.syncChoiceFields()
	m.refreshViewport()
}

// activePaletteActions returns the action tree the "/" palette should show:
// workspace-scoped actions while inside ANY registered workspace (slug
// generic — the check is registry membership, not a hardcoded catalog name),
// the global assistant actions otherwise. workspaceActions (commands.go,
// recursos-maestro design §7) builds each workspace's own "Crear ..." leaf
// from its own WorkspaceDescriptor.CreateLabel — the per-descriptor
// replacement for the old single shared materialsActions var. Reused
// everywhere the palette (re)builds its action list so backspacing to an
// empty query or retyping "/" never silently falls back to the wrong tree.
func (m Model) activePaletteActions() []assistantAction {
	if slot, ok := m.workspaces[m.activeWorkspace]; ok {
		return workspaceActions(slot.descriptor)
	}
	return assistantActions
}

func (m *Model) openPalette(query string) {
	m.heroActive = false
	m.paletteFlat = query != ""
	if query == "" {
		m.paletteActions = m.activePaletteActions()
	} else {
		m.paletteActions = flattenLeafActions(m.activePaletteActions())
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
				m.paletteActions = m.activePaletteActions()
			}
		} else if !m.paletteFlat {
			m.paletteFlat = true
			m.paletteActions = flattenLeafActions(m.activePaletteActions())
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
			if _, ok := m.workspaces[action.id]; ok {
				// A workspace's palette action id IS its registry slug
				// (WorkspaceDescriptor.Slug). Close the palette itself, mode
				// included, before snapshotting the Assistant's state:
				// enterWorkspace snapshots whatever is on Model right now, so
				// leaving interactionMode at interactionModePalette here
				// would wrongly persist "palette open" into the Assistant's
				// restored state. A confirmed selection also consumes the
				// typed "/query", unlike Esc-cancel (handled above) which
				// intentionally preserves it as a resumable draft.
				m.paletteQuery, m.paletteIndex, m.paletteActions = "", 0, nil
				m.paletteTitle = ""
				m.paletteFlat = false
				m.input = ""
				m.interactionMode = interactionModeChat
				m.enterWorkspace(action.id)
			} else {
				m.paletteQuery, m.paletteIndex, m.paletteActions = "", 0, nil
				m.paletteTitle = ""
				m.paletteFlat = false
				m.input = ""
				m.interactionMode = interactionModeChat
				m.respond(InteractionInput{Kind: InputAction, ActionID: action.id, Value: action.id, Target: ActionTargetAgent})
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
				m.paletteActions = flattenLeafActions(m.activePaletteActions())
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

func (m *Model) resetPending() {
	m.pending = nil
	m.workspacePending = false
	m.interactionMode = interactionModeChat
	m.choiceIndex = 0
	m.searchQuery = ""
	m.choiceSelected = nil
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
	allowCustom := m.pendingAllowsCustom()
	switch key {
	case "up":
		if m.choiceIndex > 0 {
			m.choiceIndex--
			m.refreshViewport()
		}
		return
	case "down":
		if m.choiceIndex < len(options)-1 {
			m.choiceIndex++
			m.refreshViewport()
		}
		return
	case "k":
		// Freed as a literal character when AllowCustom is true; falls
		// through to the printable-text append below.
		if !allowCustom {
			if m.choiceIndex > 0 {
				m.choiceIndex--
				m.refreshViewport()
			}
			return
		}
	case "j":
		if !allowCustom {
			if m.choiceIndex < len(options)-1 {
				m.choiceIndex++
				m.refreshViewport()
			}
			return
		}
	case " ", "space":
		if !allowCustom {
			m.toggleChoice()
			return
		}
	case "esc":
		if allowCustom {
			m.input = ""
		}
		m.appendCancelledInteraction(pendingQuestion(m.pending))
		m.respond(InteractionInput{Kind: InputCancel})
		return
	case "enter":
		if request, ok := m.pending.(QuestionRequest); ok && request.SelectionMode == SelectionMultiple {
			if !m.multipleSelectionValid(request) {
				m.refreshViewport()
				return
			}
			values := selectedOptionValues(options, m.choiceSelected)
			labels := selectedOptionLabels(options, m.choiceSelected)
			m.appendResolvedInteraction(pendingQuestion(m.pending), strings.Join(labels, ", "))
			m.respond(InteractionInput{Kind: InputSelection, Key: request.Key, Values: values})
			return
		}
		// Dual Enter: with AllowCustom, a non-empty typed value wins over the
		// focused option; an empty input falls back to confirming the focus.
		if allowCustom {
			if trimmed := strings.TrimSpace(m.input); trimmed != "" {
				m.appendResolvedInteraction(pendingQuestion(m.pending), trimmed)
				input := InteractionInput{Kind: inputKindFor(m.pending), Key: pendingKey(m.pending), Value: trimmed}
				m.input = ""
				m.respond(input)
				return
			}
		}
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
		return
	case "backspace":
		if allowCustom {
			m.input = trimLastRune(m.input)
		}
		return
	}
	if allowCustom && msg.Text != "" && !strings.ContainsAny(msg.Text, "\x00\x1b") {
		m.input += msg.Text
	}
}

func (m *Model) handleSearchableKey(msg tea.KeyPressMsg) {
	key := msg.String()
	allowCustom := m.pendingAllowsCustom()
	if key == "esc" {
		m.searchQuery = ""
		if allowCustom {
			m.input = ""
		}
		// screenManual (manual material search) is out of scope for the inline
		// question rendering in this PR: it must keep its existing behavior of
		// not adding a cancellation entry to the chat history.
		if m.screen != screenManual {
			m.appendCancelledInteraction(pendingQuestion(m.pending))
		}
		m.respond(InteractionInput{Kind: InputCancel})
		return
	}
	// AllowCustom frees j/k as literal characters (handled by the printable
	// text branch below); only up/down remain navigation keys in that mode.
	if key == "up" || (key == "k" && !allowCustom) {
		if m.choiceIndex > 0 {
			m.choiceIndex--
			m.refreshViewport()
		}
		return
	}
	if key == "down" || (key == "j" && !allowCustom) {
		options := m.pendingOptions()
		if m.choiceIndex < len(options)-1 {
			m.choiceIndex++
			m.refreshViewport()
		}
		return
	}
	if key == "enter" {
		// Dual Enter (spec, literal wording): with AllowCustom, typed text in
		// m.input wins over the visually-focused filtered option when
		// non-empty; this is an intentional, flagged UX follow-up, not an
		// accident of the shared buffer.
		if allowCustom {
			if trimmed := strings.TrimSpace(m.input); trimmed != "" {
				m.appendResolvedInteraction(pendingQuestion(m.pending), trimmed)
				m.input = ""
				m.respond(InteractionInput{Kind: InputSelection, Key: pendingKey(m.pending), Value: trimmed})
				return
			}
		}
		options := m.pendingOptions()
		if len(options) == 0 {
			return
		}
		selected := options[m.choiceIndex]
		m.appendResolvedInteraction(pendingQuestion(m.pending), selected.Label)
		m.searchQuery = ""
		m.input = ""
		if m.screen == screenManual {
			m.appendGARFEX(StructuredResult{Title: "Material seleccionado", Fields: []Field{{Label: "Material", Value: selected.Label}}})
			m.leaveManual()
			return
		}
		m.respond(InteractionInput{Kind: InputSelection, Key: pendingKey(m.pending), Value: selected.Value})
		return
	}
	if key == "backspace" {
		if allowCustom {
			m.input = trimLastRune(m.input)
		} else {
			m.searchQuery = trimLastRune(m.searchQuery)
		}
		m.syncChoiceFields()
		m.refreshViewport()
		return
	}
	if msg.Text != "" && !strings.ContainsAny(msg.Text, "\x00\x1b") {
		if allowCustom {
			m.input += msg.Text
		} else {
			m.searchQuery += msg.Text
		}
		m.syncChoiceFields()
		m.refreshViewport()
	}
}

func (m *Model) toggleChoice() {
	request, ok := m.pending.(QuestionRequest)
	if !ok || request.SelectionMode != SelectionMultiple || m.choiceIndex >= len(m.choiceSelected) {
		return
	}
	if !m.choiceSelected[m.choiceIndex] && request.MaxSelections > 0 && m.selectedCount() >= request.MaxSelections {
		m.refreshViewport()
		return
	}
	m.choiceSelected[m.choiceIndex] = !m.choiceSelected[m.choiceIndex]
	m.refreshViewport()
}

func (m Model) selectedCount() int {
	count := 0
	for _, selected := range m.choiceSelected {
		if selected {
			count++
		}
	}
	return count
}

func (m Model) multipleSelectionValid(request QuestionRequest) bool {
	count := m.selectedCount()
	minimum := request.MinSelections
	if minimum < 0 {
		minimum = 0
	}
	if request.MaxSelections > 0 && minimum > request.MaxSelections {
		return false
	}
	return count >= minimum && (request.MaxSelections <= 0 || count <= request.MaxSelections)
}

func selectedOptionValues(options []Option, selected []bool) []string {
	values := make([]string, 0, len(options))
	for i, option := range options {
		if i < len(selected) && selected[i] {
			values = append(values, option.Value)
		}
	}
	return values
}

func selectedOptionLabels(options []Option, selected []bool) []string {
	labels := make([]string, 0, len(options))
	for i, option := range options {
		if i < len(selected) && selected[i] {
			labels = append(labels, option.Label)
		}
	}
	return labels
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
		if m.pendingAllowsCustom() {
			return filterOptions(pendingOptionsFor(m.pending), m.input)
		}
		return filterOptions(pendingOptionsFor(m.pending), m.searchQuery)
	}
	return pendingOptionsFor(m.pending)
}

// pendingAllowsCustom is the sole key-routing discriminant between the
// existing navigate/toggle/confirm behavior (AllowCustom == false) and free
// text entry (AllowCustom == true): printable keys and j/k route to m.input
// instead of navigating, while arrows keep navigating in both modes.
func (m Model) pendingAllowsCustom() bool {
	request, ok := m.pending.(QuestionRequest)
	return ok && request.AllowCustom
}

// filterOptions keeps every option whose Label OR SearchTerms (design R6 —
// an assistantAction's Aliases/Keywords, see commands.go's actionOptions)
// contains every whitespace-separated token in query, case-insensitively.
// SearchTerms is empty for every non-palette Option, so this is a pure
// extension of the previous label-only behavior, never a narrowing of it.
func filterOptions(options []Option, query string) []Option {
	tokens := strings.Fields(strings.ToLower(query))
	if len(tokens) == 0 {
		return append([]Option(nil), options...)
	}
	filtered := make([]Option, 0, len(options))
	for _, option := range options {
		haystack := strings.ToLower(option.Label)
		if len(option.SearchTerms) > 0 {
			haystack += " " + strings.ToLower(strings.Join(option.SearchTerms, " "))
		}
		matches := true
		for _, token := range tokens {
			if !strings.Contains(haystack, token) {
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

func (m *Model) appendCancelledInteraction(prompt string) {
	wasAtBottom := m.viewport.AtBottom()
	m.history = append(m.history, conversationMessage{
		role:     roleGARFEX,
		resolved: &resolvedInteraction{prompt: prompt, cancelled: true},
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
	if request, ok := m.pending.(QuestionRequest); ok && request.SelectionMode == SelectionMultiple {
		m.choiceSelected = make([]bool, len(options))
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
	m.choiceSelected = nil
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
	wasAtBottom := m.viewport.AtBottom()
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
			query := m.searchQuery
			if m.pendingAllowsCustom() {
				query = m.input
			}
			lines = append(lines, renderActiveSearchable(m.pending, query, m.choiceIndex), "")
		} else {
			lines = append(lines, m.renderActiveInteraction(m.pending, m.choiceIndex), "")
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
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

func (m Model) interactionDockLines(width int) int {
	return max(1, lipgloss.Height(m.renderInteractionDock(width)))
}

func renderResolvedInteraction(interaction resolvedInteraction) string {
	if interaction.cancelled {
		return interaction.prompt + "\n" + lipgloss.NewStyle().Foreground(secondaryText).Render("· cancelado")
	}
	return interaction.prompt + "\n✓ " + interaction.selection
}

func renderActiveInteraction(message InteractionMessage, selected int) string {
	return renderActiveInteractionWithSelection(message, selected, nil)
}

func (m Model) renderActiveInteraction(message InteractionMessage, selected int) string {
	return renderActiveInteractionWithSelection(message, selected, m.choiceSelected)
}

func renderActiveInteractionWithSelection(message InteractionMessage, selected int, selection []bool) string {
	lines := []string{renderInteractionMessage(message)}
	options := pendingOptionsFor(message)
	if request, ok := message.(QuestionRequest); ok && request.SelectionMode == SelectionMultiple {
		selectedState := make([]bool, len(options))
		if len(selection) == len(options) {
			selectedState = selection
		}
		lines = append(lines, selectionStatus(request, countSelected(selectedState)))
		for i, option := range options {
			lines = append(lines, renderMultipleOption(option, i == selected, i < len(selectedState) && selectedState[i]))
		}
		return strings.Join(lines, "\n")
	}
	for i, option := range options {
		active := i == selected
		label := "  " + option.Label
		if active {
			label = "❯ " + option.Label
		}
		lines = append(lines, interactionOptionStyle(active, false).Render(label))
		if option.Description != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(secondaryText).Render("    "+option.Description))
		}
	}
	return strings.Join(lines, "\n")
}

func countSelected(selected []bool) int {
	count := 0
	for _, value := range selected {
		if value {
			count++
		}
	}
	return count
}

func selectionStatus(request QuestionRequest, count int) string {
	status := fmt.Sprintf("Seleccionadas: %d", count)
	if request.MinSelections > 0 {
		status += fmt.Sprintf(" · Mínimo: %d", request.MinSelections)
	}
	if request.MaxSelections > 0 {
		status += fmt.Sprintf(" · Máximo: %d", request.MaxSelections)
	} else {
		status += " · Sin máximo"
	}
	minimum := request.MinSelections
	if minimum < 0 {
		minimum = 0
	}
	if request.MaxSelections > 0 && minimum > request.MaxSelections {
		status += " · Restricciones inválidas"
	} else if count < minimum {
		status += " · Faltan selecciones"
	} else if request.MaxSelections > 0 && count == request.MaxSelections {
		status += " · Máximo alcanzado"
	}
	return status
}

func renderMultipleOption(option Option, focused, selected bool) string {
	marker := "  "
	if focused {
		marker = "❯ "
	}
	check := "[ ]"
	if selected {
		check = "[x]"
	}
	line := interactionOptionStyle(focused, selected).Render(marker + check + " " + option.Label)
	if option.Description == "" {
		return line
	}
	return line + "\n" + lipgloss.NewStyle().Foreground(secondaryText).Render("    "+option.Description)
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
func (m Model) View() tea.View {
	if m.screen == screenMinSize {
		return tea.NewView("La terminal debe tener al menos 40x10.")
	}
	return tea.NewView(m.render())
}
