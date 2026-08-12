package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func key(code rune) tea.KeyPressMsg  { return tea.KeyPressMsg(tea.Key{Code: code, Text: string(code)}) }
func enter() tea.KeyPressMsg         { return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}) }
func normalized(value string) string { return strings.Join(strings.Fields(value), " ") }
func containsFullWordmark(value string) bool {
	for _, line := range strings.Split(fullWordmark, "\n") {
		if !strings.Contains(value, strings.TrimSpace(line)) {
			return false
		}
	}
	return true
}
func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func TestModelMenuAndCommands(t *testing.T) {
	called := false
	handlers := Handlers{Version: func() tea.Cmd { called = true; return func() tea.Msg { return resultMsg{text: "ok"} } }, Config: Status(), Status: Status()}
	m := New(handlers)
	if got := ansi.Strip(m.View().Content); containsFullWordmark(got) || !strings.Contains(got, "GARFEX") || !strings.Contains(normalized(got), officialTagline) || !strings.Contains(got, "› 01  Materiales Maestros") || !strings.Contains(got, "02  Versión") || !strings.Contains(got, "03  Verificar configuración") || !strings.Contains(got, "04  Estado de GARFEX") || !strings.Contains(got, "05  Salir") || !strings.Contains(got, "flechas") || strings.Contains(got, "/\\/") || strings.Contains(got, "╭") {
		t.Fatalf("menu = %q", got)
	}
	for range 8 {
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	}
	if m.cursor != 0 {
		t.Fatalf("upper cursor = %d", m.cursor)
	}
	for range 8 {
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	}
	if m.cursor != 4 {
		t.Fatalf("lower cursor = %d", m.cursor)
	}
	m.items[m.cursor].Handler = func() tea.Cmd { called = true; return nil }
	_, quit := update(t, m, enter())
	if quit == nil || called {
		t.Fatal("localized exit must quit without calling a handler")
	}
	m, _ = update(t, New(handlers), tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, cmd := update(t, m, enter())
	if m.screen != screenLoading || !called || cmd == nil {
		t.Fatalf("loading=%v called=%v cmd=%v", m.screen, called, cmd)
	}
	if _, quit = update(t, New(handlers), tea.KeyPressMsg(tea.Key{Code: 'q'})); quit != nil {
		t.Fatal("reserved letters must not quit from Home")
	}
	if _, quit = update(t, New(handlers), tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})); quit == nil {
		t.Fatal("ctrl+c must quit")
	}
}

func TestModelResultsSizingAndView(t *testing.T) {
	m := New(Handlers{Version: Status(), Config: Status(), Status: Status()})
	m.cursor = 2
	m, _ = update(t, m, resultMsg{text: "ok\x1b"})
	if got := ansi.Strip(m.View().Content); m.screen != screenResult || strings.Contains(got, "\x1b") {
		t.Fatal("success must be sanitized")
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.screen != screenHome || m.cursor != 2 {
		t.Fatal("back must preserve cursor")
	}
	m, _ = update(t, m, resultMsg{err: errors.New("bad\x7f")})
	if got := ansi.Strip(m.View().Content); m.screen != screenError || strings.Contains(got, "\x7f") {
		t.Fatal("error must be sanitized")
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 39, Height: 10}, {Width: 40, Height: 9}} {
		m, _ = update(t, m, size)
		if m.screen != screenMinSize {
			t.Fatalf("size %+v accepted", size)
		}
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 10})
	if m.screen != screenHome {
		t.Fatal("valid resize must recover home")
	}
	if one, two := m.View().Content, m.View().Content; one != two {
		t.Fatal("view must be deterministic")
	}
}

func TestModelMissingContracts(t *testing.T) {
	m := New(Handlers{Version: Status(), Config: Status(), Status: Status()})
	if len(m.items) != 5 || m.items[0].Label != "Materiales Maestros" || m.items[1].Label != "Versión" || m.items[2].Label != "Verificar configuración" || m.items[3].Label != "Estado de GARFEX" || m.items[4].Label != "Salir" || !m.items[4].Quit {
		t.Fatalf("items = %#v", m.items)
	}
	for _, tt := range []struct {
		msg  tea.Msg
		want int
	}{{tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), 1}, {tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), 0}} {
		m, _ = update(t, m, tt.msg)
		if m.cursor != tt.want {
			t.Fatalf("cursor = %d, want %d", m.cursor, tt.want)
		}
	}
	if _, quit := update(t, m, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})); quit == nil {
		t.Fatal("ctrl+c must quit")
	}
	for _, tt := range []struct {
		msg  resultMsg
		want string
		s    screen
	}{{resultMsg{text: "a\x00\n\r\t\x1b\x7fb"}, "a\n\r\tb", screenResult}, {resultMsg{err: errors.New("a\x00\n\r\t\x1b\x7fb")}, "a\n\r\tb", screenError}} {
		m, _ = update(t, m, tt.msg)
		if m.result != tt.want || m.screen != tt.s {
			t.Fatalf("result = %q, screen = %v", m.result, m.screen)
		}
	}
	m, cmd := update(t, m, enter())
	if m.screen != screenLoading || cmd == nil {
		t.Fatal("error retry must load and return a command")
	}
	if m, cmd = update(t, m, enter()); m.screen != screenLoading || cmd != nil {
		t.Fatal("loading must ignore retry")
	}
	m, _ = update(t, New(Handlers{Version: Status(), Config: Status(), Status: Status()}), resultMsg{text: "ok"})
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.screen != screenHome {
		t.Fatal("esc must return to home")
	}
	for _, tt := range []struct {
		name string
		s    screen
		want []string
	}{
		{"home", screenHome, []string{"HOME · ÁREAS DE GARFEX", "› 01  Materiales Maestros", "05  Salir", "flechas", "enter elegir", "ctrl+c salir"}},
		{"workspace", screenWorkspace, []string{"GARFEX / MATERIALES MAESTROS", "Buscar material", "esc volver", "ctrl+c salir"}},
		{"loading", screenLoading, []string{"Trabajando", "Buscando material...", "Esperá un momento", "esc cancelar"}},
		{"success", screenResult, []string{"Resultado", "ok", "enter reintentar", "esc volver"}},
		{"error", screenError, []string{"No se pudo completar la consulta", "bad", "enter reintentar", "esc volver"}},
		{"minimum", screenMinSize, []string{"La terminal debe tener al menos 40x10."}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m.width, m.height = 102, 24
			m.screen, m.result = tt.s, "bad"
			if tt.s == screenResult {
				m.result = "ok"
			}
			one, two := m.View().Content, m.View().Content
			if one != two {
				t.Fatalf("%s view is not deterministic", tt.name)
			}
			plain := ansi.Strip(one)
			for _, want := range tt.want {
				if !strings.Contains(plain, want) {
					t.Fatalf("%s view = %q, missing %q", tt.name, plain, want)
				}
			}
		})
	}
}

func TestModelViewResponsiveShell(t *testing.T) {
	for _, tt := range []struct {
		name          string
		width, height int
		full          bool
	}{
		{"full", 120, 30, true},
		{"exact banner width", 102, 24, true},
		{"exact full height", 102, 21, false},
		{"insufficient full height", 102, 20, false},
		{"ordinary terminal", 80, 24, false},
		{"minimum boundary", 40, 10, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Handlers{Version: Status(), Config: Status(), Status: Status()})
			m, _ = update(t, m, tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			got := m.View().Content
			wantHeight := tt.height
			if tt.name == "minimum boundary" {
				wantHeight = 11
			}
			if lipgloss.Width(got) != tt.width || lipgloss.Height(got) != wantHeight {
				t.Fatalf("view size = %dx%d, want %dx%d", lipgloss.Width(got), lipgloss.Height(got), tt.width, wantHeight)
			}
			plain := ansi.Strip(got)
			if !strings.Contains(normalized(plain), officialTagline) || strings.Contains(plain, "/\\/") || strings.Contains(plain, "╭") {
				t.Fatalf("shell is not centered or branded: %q", plain)
			}
			if tt.full != containsFullWordmark(plain) {
				t.Fatalf("full wordmark presence = %v, want %v", containsFullWordmark(plain), tt.full)
			}
			if !tt.full && (!strings.Contains(plain, "GARFEX") || strings.Contains(plain, "G A R F E X") || strings.Contains(plain, "MENÚ PRINCIPAL")) {
				t.Fatalf("compact shell fallback is invalid: %q", plain)
			}
		})
	}
}

func TestFullWordmark(t *testing.T) {
	const (
		wantRows    = 9
		wantColumns = 100
	)
	if maxCardWidth != wantColumns {
		t.Fatalf("maximum card width = %d, want %d", maxCardWidth, wantColumns)
	}
	wordmarkLines := strings.Split(fullWordmark, "\n")
	if len(wordmarkLines) != wantRows {
		t.Fatalf("wordmark lines = %d, want %d", len(wordmarkLines), wantRows)
	}
	for row, line := range wordmarkLines {
		if got := lipgloss.Width(line); got != wantColumns {
			t.Fatalf("wordmark row %d width = %d, want %d", row, got, wantColumns)
		}
		for column, glyph := range []rune(line) {
			if glyph < '\u2800' || glyph > '\u28ff' {
				t.Fatalf("wordmark row %d column %d contains non-Braille glyph %q", row, column, glyph)
			}
			if got := lipgloss.Width(string(glyph)); got != 1 {
				t.Fatalf("wordmark glyph %q width = %d, want 1", glyph, got)
			}
		}
	}

	styled := wordmarkStyle(lipgloss.Width(fullWordmark)).GetForeground()
	wantR, wantG, wantB, wantA := brandRed.RGBA()
	gotR, gotG, gotB, gotA := styled.RGBA()
	if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
		t.Fatal("full wordmark does not use GARFEX corporate red")
	}

	m := New(Handlers{Version: Status(), Config: Status(), Status: Status()})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 102, Height: 24})
	one, two := m.View().Content, m.View().Content
	if one != two {
		t.Fatal("full wordmark render must be deterministic")
	}
	plain := ansi.Strip(one)
	lines := strings.Split(plain, "\n")
	first := -1
	for i, line := range lines {
		if strings.Contains(line, wordmarkLines[0]) {
			first = i
			break
		}
	}
	if first < 0 {
		t.Fatal("rendered view is missing the full wordmark")
	}
	last := first + len(wordmarkLines) - 1
	for row, want := range wordmarkLines {
		if !strings.Contains(lines[first+row], want) {
			t.Fatalf("rendered wordmark row %d = %q, missing %q", row, lines[first+row], want)
		}
	}
	tagline := -1
	for i := last + 1; i < len(lines); i++ {
		if strings.Contains(lines[i], "DISEÑO") {
			tagline = i
			break
		}
		if strings.TrimSpace(lines[i]) != "" {
			t.Fatalf("unexpected content between wordmark and tagline: %q", lines[i])
		}
	}
	if tagline <= last+1 {
		t.Fatalf("wordmark/tagline spacing is invalid: first=%d last=%d", first, last)
	}
	for row, want := range wordmarkLines {
		line := lines[first+row]
		start := strings.Index(line, want)
		left := lipgloss.Width(line[:start])
		right := lipgloss.Width(line) - left - lipgloss.Width(want)
		if left-right < -1 || left-right > 1 {
			t.Fatalf("wordmark row %d is not centered: left=%d right=%d", row, left, right)
		}
	}
}

func TestBrandPalette(t *testing.T) {
	if backgroundHex != "#0B0D0E" || surfaceHex != "#17191B" || primaryTextHex != "#F2F0E9" || secondaryTextHex != "#8B8B86" || brandRedHex != "#800000" || accentHex != "#FFD400" {
		t.Fatalf("palette = %s, %s, %s, %s, %s, %s", backgroundHex, surfaceHex, primaryTextHex, secondaryTextHex, brandRedHex, accentHex)
	}
	if brandRedHex == accentHex {
		t.Fatal("banner red and interaction accent must remain distinct")
	}
	if successHex == accentHex || errorHex == accentHex {
		t.Fatal("success and error colors must remain semantic")
	}
	if stateAccent(screenLoading) != accentHex || stateAccent(screenResult) != successHex || stateAccent(screenError) != errorHex {
		t.Fatal("state colors do not match their semantic roles")
	}
	if _, ok := menuItemStyle(true).GetBackground().(lipgloss.NoColor); !ok {
		t.Fatal("active menu item must not use a background fill")
	}
}

func TestWorkspaceInputAndHistory(t *testing.T) {
	m := New(Handlers{})
	m, _ = update(t, m, enter())
	if m.screen != screenWorkspace || m.inputFocused {
		t.Fatalf("workspace entry = (%v, %v)", m.screen, m.inputFocused)
	}
	m, _ = update(t, m, enter())
	for _, char := range []rune("búscame cable 10 negro") {
		m, _ = update(t, m, key(char))
	}
	message := m.input
	m, cmd := update(t, m, enter())
	if m.screen != screenWorkspace || m.inputFocused || m.input != "" || len(m.history) != 1 || m.history[0].text != message || m.interactionMode != interactionModeChoice || cmd != nil {
		t.Fatalf("submit = screen %v, focused %v, input %q, history %#v", m.screen, m.inputFocused, m.input, m.history)
	}
	if m.history[0].text != message || m.choicePrompt != "¿Qué aislamiento necesitas?" {
		t.Fatalf("choice history = %#v", m.history)
	}
	for _, forbidden := range []string{"PROCESSING", "RESULT", "VOS:", "Historial:", "HISTORIAL ·"} {
		if strings.Contains(ansi.Strip(m.View().Content), forbidden) {
			t.Fatalf("workspace leaked internal label %q: %q", forbidden, ansi.Strip(m.View().Content))
		}
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m, _ = update(t, m, enter())
	if !m.inputFocused || m.input != "" || len(m.history) != 2 || m.history[len(m.history)-1].text != "Está bien, volvemos a la conversación." {
		t.Fatalf("history refocus = input %q, focused %v, view %q", m.input, m.inputFocused, ansi.Strip(m.View().Content))
	}
}

func TestFocusedInputPreservesReservedLettersAndDraft(t *testing.T) {
	m := New(Handlers{})
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	for _, char := range []rune{'b', 'q', 'r', 'j', 'k'} {
		m, _ = update(t, m, key(char))
	}
	if m.input != "bqrjk" || !m.inputFocused || m.screen != screenWorkspace {
		t.Fatalf("reserved letters = (%q, %v, %v)", m.input, m.inputFocused, m.screen)
	}
	m, _ = update(t, m, key('x'))
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	if m.input != "bqrjk" {
		t.Fatalf("backspace input = %q", m.input)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = New(Handlers{})
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	for _, char := range []rune("nuevo draft") {
		m, _ = update(t, m, key(char))
	}
	if m.input != "nuevo draft" || len(m.history) != 0 {
		t.Fatalf("draft = input %q, history %#v", m.input, m.history)
	}
}

func TestEmptyInputDoesNotAddHistory(t *testing.T) {
	m := New(Handlers{})
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	m, cmd := update(t, m, enter())
	if len(m.history) != 0 || m.input != "" || !m.inputFocused || m.screen != screenWorkspace || m.workspaceState != screenHome || cmd != nil {
		t.Fatalf("empty submit = screen %v, input %q, history %#v, cmd=%v", m.screen, m.input, m.history, cmd)
	}
	for _, char := range []rune("cable 10 negro") {
		m, _ = update(t, m, key(char))
	}
	m, cmd = update(t, m, enter())
	if cmd != nil || m.screen != screenWorkspace || m.inputFocused || m.input != "" || len(m.history) != 1 || m.interactionMode != interactionModeChoice {
		t.Fatalf("new instruction = screen %v, state %v, focused %v, input %q, history %#v, cmd=%v", m.screen, m.workspaceState, m.inputFocused, m.input, m.history, cmd)
	}
}

func TestPromptHistoryNavigationIsSeparateFromConversationHistory(t *testing.T) {
	m := workspaceChat(t)
	m = submitText(t, m, "primera consulta")
	m, _ = update(t, m, enter())
	m = submitText(t, m, "segunda consulta")
	if len(m.promptHistory) != 2 || len(m.history) != 4 {
		t.Fatalf("prompt history=%#v conversation history=%d", m.promptHistory, len(m.history))
	}
	m, _ = update(t, m, enter())
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.input != "segunda consulta" {
		t.Fatalf("up input=%q", m.input)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.input != "primera consulta" {
		t.Fatalf("second up input=%q", m.input)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.input != "segunda consulta" {
		t.Fatalf("down input=%q", m.input)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.input != "" || m.promptHistoryCursor != -1 {
		t.Fatalf("final down input=%q cursor=%d", m.input, m.promptHistoryCursor)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	if m.input != "" {
		t.Fatalf("page up changed draft=%q", m.input)
	}
}

func TestSelectorKeysDoNotNavigatePromptHistoryOrViewport(t *testing.T) {
	m := workspaceChat(t)
	m.promptHistory = []string{"previous"}
	m.promptHistoryCursor = -1
	m.input = "draft"
	m.setPendingQuestion("¿Qué opción?", []string{"Uno", "Dos"})
	position := m.viewport.YOffset()
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.choiceIndex != 1 || m.promptHistoryCursor != -1 || m.input != "draft" || m.viewport.YOffset() != position {
		t.Fatalf("selector down changed unrelated state: index=%d cursor=%d input=%q offset=%d", m.choiceIndex, m.promptHistoryCursor, m.input, m.viewport.YOffset())
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	if m.choiceIndex != 1 || m.promptHistoryCursor != -1 {
		t.Fatalf("page up changed selection: index=%d cursor=%d", m.choiceIndex, m.promptHistoryCursor)
	}
}

func TestWorkspaceShortcutsUseLocalIDs(t *testing.T) {
	agent := &recordingAgent{}
	m := NewWithAgent(Handlers{}, agent)
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	if !m.inputFocused {
		t.Fatal("search shortcut did not focus chat")
	}
	if agent.calls != 0 {
		t.Fatalf("shortcut called agent %d times", agent.calls)
	}
}

func TestWorkspaceConversationScrollAndDraftAreIndependent(t *testing.T) {
	m := New(Handlers{})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	m.screen = screenWorkspace
	m.inputFocused = true
	for i := 0; i < 12; i++ {
		m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "turno largo con suficiente contenido para envolver líneas"})
	}
	if !m.viewport.AtBottom() {
		t.Fatal("new messages must initially stay at the bottom")
	}
	m.input = "draft independiente"
	for range 4 {
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	}
	if m.viewport.AtBottom() {
		t.Fatal("up must scroll the history")
	}
	if m.input != "draft independiente" {
		t.Fatalf("scroll changed draft: %q", m.input)
	}
	position := m.viewport.YOffset()
	m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "mensaje nuevo mientras se lee arriba"})
	if m.viewport.YOffset() != position {
		t.Fatal("new message forced a jump while reading history")
	}
	for range 30 {
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	}
	if !m.viewport.AtBottom() {
		t.Fatal("down must stop at the bottom")
	}
	m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "mensaje que sigue al retorno"})
	if !m.viewport.AtBottom() {
		t.Fatal("messages must auto-scroll after returning to the bottom")
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	if m.viewport.AtBottom() {
		t.Fatal("page up must leave the bottom")
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	if !m.viewport.AtBottom() {
		t.Fatal("page down must reach the bottom")
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "draft independiente") {
		t.Fatalf("workspace view lost fixed input: %q", plain)
	}
}

func TestWorkspaceConversationScrollsWhenChatIsNotFocused(t *testing.T) {
	m := New(Handlers{})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	m.screen = screenWorkspace
	for i := 0; i < 12; i++ {
		m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "turno largo para desplazar la conversación"})
	}
	if m.inputFocused {
		t.Fatal("test setup unexpectedly focused chat input")
	}
	m.workspaceItem = 1
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	if m.viewport.AtBottom() || m.workspaceItem != 1 {
		t.Fatalf("page up changed wrong state: offset=%d workspace item=%d", m.viewport.YOffset(), m.workspaceItem)
	}
	position := m.viewport.YOffset()
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.workspaceItem != 0 || m.viewport.YOffset() != position {
		t.Fatalf("up did not navigate workspace: item=%d offset=%d", m.workspaceItem, m.viewport.YOffset())
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.workspaceItem != 1 || m.viewport.YOffset() != position {
		t.Fatalf("down did not navigate workspace: item=%d offset=%d", m.workspaceItem, m.viewport.YOffset())
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	if !m.viewport.AtBottom() || m.workspaceItem != 1 {
		t.Fatalf("page down changed wrong state: offset=%d workspace item=%d", m.viewport.YOffset(), m.workspaceItem)
	}
}

func TestWorkspaceConversationHomeEndWhenChatIsNotFocused(t *testing.T) {
	m := New(Handlers{})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	m.screen = screenWorkspace
	for i := 0; i < 12; i++ {
		m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "turno largo para desplazar la conversación"})
	}
	if m.inputFocused {
		t.Fatal("test setup unexpectedly focused chat input")
	}
	if !m.viewport.AtBottom() {
		t.Fatal("new messages must initially stay at the bottom")
	}

	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}))
	if !m.viewport.AtTop() {
		t.Fatalf("home did not move conversation to the top: offset=%d", m.viewport.YOffset())
	}

	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	if !m.viewport.AtBottom() {
		t.Fatalf("end did not move conversation to the bottom: offset=%d", m.viewport.YOffset())
	}
	m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "mensaje que sigue al retorno"})
	if !m.viewport.AtBottom() {
		t.Fatal("messages must auto-scroll after returning to the bottom")
	}
}

func TestWorkspaceResizeAndOperationMessages(t *testing.T) {
	m := New(Handlers{})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 70, Height: 24})
	m.screen = screenWorkspace
	m.appendMessage(conversationMessage{role: roleUser, kind: kindText, text: "consulta"})
	m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindProgress, text: "Procesando"})
	if m.workspaceState != screenHome {
		t.Fatal("idle conversation should not report a terminal state")
	}
	m.workspaceState = screenLoading
	m.workspacePending = true
	m, _ = update(t, m, resultMsg{text: "resultado simulado"})
	if m.workspaceState != screenResult || m.history[len(m.history)-1].kind != kindResult {
		t.Fatalf("result state = %v, history = %#v", m.workspaceState, m.history)
	}
	m.workspaceState = screenLoading
	m.workspacePending = true
	m, _ = update(t, m, resultMsg{err: errors.New("falló la simulación")})
	if m.workspaceState != screenError || m.history[len(m.history)-1].kind != kindError {
		t.Fatalf("error state = %v, history = %#v", m.workspaceState, m.history)
	}
	if len(m.history) != 4 {
		t.Fatalf("turn count = %d, want 4 typed messages", len(m.history))
	}
	oldHistory := m.viewport.GetContent()
	oldDraft := m.input
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 55, Height: 18})
	if m.viewport.Width() != 51 || m.viewport.Height() != workspaceViewportHeight(18, interactionModeChat) || m.viewport.GetContent() != oldHistory || m.input != oldDraft {
		t.Fatalf("resize lost state: viewport=%dx%d draft=%q", m.viewport.Width(), m.viewport.Height(), m.input)
	}
}

func TestWorkspaceChromeStaysVisibleWhenConversationScrolls(t *testing.T) {
	m := New(Handlers{})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	m.screen = screenWorkspace
	m.inputFocused = true
	m.input = "cable THW-LS 10 AWG"
	for i := 0; i < 12; i++ {
		m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindResult, text: "La ficha técnica incluye unidad metro, color y atributos de aislación para comparar alternativas."})
	}
	for range 6 {
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	}

	plain := ansi.Strip(m.View().Content)
	for _, visible := range []string{"GARFEX / MATERIALES MAESTROS", "Buscar material", "Crear material", "Catálogo", "Historial", "cable THW-LS 10 AWG"} {
		if !strings.Contains(plain, visible) {
			t.Fatalf("fixed workspace chrome lost %q after scroll: %q", visible, plain)
		}
	}
	for _, forbidden := range []string{"PROCESSING", "RESULT", "ERROR", "VOS:", "Historial:", "HISTORIAL ·"} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("workspace leaked internal label %q: %q", forbidden, plain)
		}
	}
}

func TestChoiceViewUsesSelectionControlsAndStableViewport(t *testing.T) {
	m := submitText(t, workspaceChat(t), "cable 10")
	plain := ansi.Strip(m.View().Content)
	if m.inputFocused || strings.Contains(plain, "▌") || strings.Contains(plain, "❯ escribí una consulta") {
		t.Fatalf("choice view rendered chat input: %q", plain)
	}
	for _, want := range []string{"¿Qué aislamiento necesitas?", "THW-LS", "↑↓ seleccionar · Enter confirmar · Esc cancelar", "GARFEX / MATERIALES MAESTROS", "Buscar material", "enter confirmar", "esc cancelar"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("choice view missing %q: %q", want, plain)
		}
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 10}, {Width: 60, Height: 20}, {Width: 100, Height: 30}} {
		m, _ = update(t, m, size)
		wantHeight := max(1, size.Height-fixedWorkspaceHeight-interactionDockHeight)
		if m.viewport.Height() != wantHeight {
			t.Fatalf("viewport height for %dx%d = %d, want %d", size.Width, size.Height, m.viewport.Height(), wantHeight)
		}
	}
	for i := 0; i < 12; i++ {
		m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "mensaje largo para desplazar la conversación"})
	}
	for range 6 {
		m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	}
	plain = ansi.Strip(m.View().Content)
	for _, want := range []string{"GARFEX / MATERIALES MAESTROS", "Buscar material", "enter confirmar", "esc cancelar"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("fixed choice chrome missing after scroll %q: %q", want, plain)
		}
	}
}

func workspaceChat(t *testing.T) Model {
	t.Helper()
	m := New(Handlers{})
	m, _ = update(t, m, enter())
	m, _ = update(t, m, enter())
	return m
}

func submitText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, char := range []rune(text) {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	return m
}

func TestGuidedMaterialFixtures(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPrompt string
		wantOption string
	}{
		{name: "cable asks insulation", input: "cable 10", wantPrompt: "¿Qué aislamiento necesitas?", wantOption: "THW-LS"},
		{name: "cable with color asks insulation", input: "cable 10 negro", wantPrompt: "¿Qué aislamiento necesitas?", wantOption: "THW-LS"},
		{name: "tube asks type", input: "tubería 1/2", wantPrompt: "¿Qué tipo de tubería necesitas?", wantOption: "PVC conduit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := submitText(t, workspaceChat(t), tt.input)
			if m.interactionMode != interactionModeChoice || m.choicePrompt != tt.wantPrompt || !containsString(m.choiceOptions, tt.wantOption) {
				t.Fatalf("choice = (%v, %q, %#v)", m.interactionMode, m.choicePrompt, m.choiceOptions)
			}
		})
	}
}

func TestCableGuidedFlow(t *testing.T) {
	m := submitText(t, workspaceChat(t), "cable 10")
	m, _ = update(t, m, enter())
	if m.choiceOptions[0] != "Negro" || m.choicePrompt != "¿Qué color necesitas?" {
		t.Fatalf("after insulation = prompt %q options %#v", m.choicePrompt, m.choiceOptions)
	}
	m, _ = update(t, m, enter())
	if m.choiceOptions[0] != "Buscar" || !historyContains(m.history, "Buscaré: THW-LS · 10 AWG · Negro") {
		t.Fatalf("after color = %#v, history %#v", m.choiceOptions, m.history)
	}
	m, _ = update(t, m, enter())
	plain := m.history[len(m.history)-1].text
	for _, want := range []string{"MATERIAL ENCONTRADO", "THW-LS · 10 AWG · Negro", "Voltaje: 600 V", "Unidad: m", "Precio: simulado"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("result missing %q in %q", want, plain)
		}
	}
	if strings.Contains(plain, "[") || strings.Contains(plain, "Ver precios") || strings.Contains(plain, "Proveedores") || strings.Contains(plain, "Usar en APU") {
		t.Fatalf("result duplicated pending actions: %q", plain)
	}
	if got := strings.Count(ansi.Strip(m.View().Content), "Ver precios"); got != 1 {
		t.Fatalf("dock action count = %d, want one", got)
	}
}

func TestResolvedSelectionStaysInlineBeforeNextPending(t *testing.T) {
	m := submitText(t, workspaceChat(t), "cable 10")
	plain := ansi.Strip(m.View().Content)
	for _, want := range []string{"¿Qué aislamiento necesitas?", "THW-LS", "XHHW-2", "DESNUDO"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("active question missing %q: %q", want, plain)
		}
	}
	m, _ = update(t, m, enter())
	plain = ansi.Strip(m.View().Content)
	if !strings.Contains(ansi.Strip(m.viewport.GetContent()), "¿Qué aislamiento necesitas?\n✓ THW-LS") {
		t.Fatalf("resolved interaction not compact: %q", m.viewport.GetContent())
	}
	if !strings.Contains(plain, "¿Qué color necesitas?") || strings.Count(plain, "THW-LS") != 1 {
		t.Fatalf("next pending or resolved selection rendered incorrectly: %q", plain)
	}
	if strings.Contains(plain, "XHHW-2") || strings.Contains(plain, "DESNUDO") {
		t.Fatalf("previous options remained active: %q", plain)
	}
}

func TestPendingAutoScrollShowsCompleteInlineBlock(t *testing.T) {
	m := submitText(t, workspaceChat(t), "cable 10")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	content := ansi.Strip(m.View().Content)
	for _, want := range []string{"¿Qué aislamiento necesitas?", "THW-LS", "XHHW-2", "DESNUDO"} {
		if !strings.Contains(content, want) {
			t.Fatalf("sticky bottom hid active interaction %q: %q", want, content)
		}
	}
	if !m.viewport.AtBottom() {
		t.Fatal("pending response should remain sticky at the bottom")
	}
}

func TestCableProvidedAttributesAreNotAskedAgain(t *testing.T) {
	for _, input := range []string{"cable 10 negro", "THW-LS 10 negro"} {
		t.Run(input, func(t *testing.T) {
			m := submitText(t, workspaceChat(t), input)
			if input == "cable 10 negro" {
				if m.choicePrompt != "¿Qué aislamiento necesitas?" {
					t.Fatalf("prompt = %q", m.choicePrompt)
				}
				m, _ = update(t, m, enter())
				if m.choicePrompt == "¿Qué color necesitas?" {
					t.Fatal("color was asked again")
				}
			} else if m.interactionMode != interactionModeAction || m.pending == nil || !strings.Contains(m.history[len(m.history)-1].text, "Buscaré:") {
				t.Fatalf("direct action = mode %v, history %#v", m.interactionMode, m.history)
			}
		})
	}
}

func TestTubeChoiceRendersResult(t *testing.T) {
	m := submitText(t, workspaceChat(t), "tubo 1/2")
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = update(t, m, enter())
	result := m.history[len(m.history)-1].text
	for _, want := range []string{"MATERIAL ENCONTRADO", `1/2" · Conduit pared gruesa`, "Unidad: m", "Precio: simulado"} {
		if !strings.Contains(result, want) {
			t.Fatalf("missing %q in %q", want, result)
		}
	}
}

func TestChoiceKeysDoNotScrollOrChangeDraft(t *testing.T) {
	m := workspaceChat(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	for i := 0; i < 20; i++ {
		m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "mensaje largo para llenar la conversación"})
	}
	m.input = "draft independiente"
	m.setPendingQuestion("¿Qué opción?", []string{"Uno", "Dos"})
	position := m.viewport.YOffset()
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.choiceIndex != 1 || m.viewport.YOffset() != position || m.input != "draft independiente" {
		t.Fatalf("down changed unrelated state")
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.choiceIndex != 0 || m.viewport.YOffset() != position {
		t.Fatalf("up changed scroll")
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.interactionMode != interactionModeChat || m.input != "draft independiente" {
		t.Fatalf("esc changed draft or mode")
	}
}

func TestPendingModesRenderSelectorControlsAndKeepViewportFixed(t *testing.T) {
	tests := []struct {
		name  string
		model func(*testing.T) Model
	}{
		{name: "confirmation", model: func(t *testing.T) Model {
			m := submitText(t, workspaceChat(t), "crear material")
			m, _ = update(t, m, enter())
			return m
		}},
		{name: "action", model: func(t *testing.T) Model {
			m := submitText(t, workspaceChat(t), "cable 10")
			m, _ = update(t, m, enter())
			m, _ = update(t, m, enter())
			m, _ = update(t, m, enter())
			return m
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.model(t)
			m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
			for i := 0; i < 20; i++ {
				m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "mensaje largo para llenar la conversación"})
			}
			position := m.viewport.YOffset()
			plain := ansi.Strip(m.View().Content)
			for _, want := range []string{"↑↓ seleccionar", "enter confirmar", "esc cancelar"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("missing control %q: %q", want, plain)
				}
			}
			for _, forbidden := range []string{"PROCESSING", "RESULT", "MessageType", "VOS", "▌", "❯ escribí una consulta"} {
				if strings.Contains(plain, forbidden) {
					t.Fatalf("pending view leaked %q: %q", forbidden, plain)
				}
			}
			m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
			if m.choiceIndex != 1 || m.viewport.YOffset() != position {
				t.Fatalf("down changed selector/viewport: index=%d offset=%d want=%d", m.choiceIndex, m.viewport.YOffset(), position)
			}
			m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
			if m.choiceIndex != 0 || m.viewport.YOffset() != position {
				t.Fatalf("up changed selector/viewport: index=%d offset=%d want=%d", m.choiceIndex, m.viewport.YOffset(), position)
			}
			m, _ = update(t, m, enter())
			if tt.name == "confirmation" && (m.interactionMode != interactionModeChat || m.pending != nil) {
				t.Fatalf("enter did not confirm selector: mode=%v pending=%T", m.interactionMode, m.pending)
			}
			if tt.name == "action" && (m.interactionMode != interactionModeAction || m.pending == nil) {
				t.Fatalf("action result lost its action request: mode=%v pending=%T", m.interactionMode, m.pending)
			}

			m = tt.model(t)
			m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
			if m.interactionMode != interactionModeChat || m.pending != nil || m.inputFocused {
				t.Fatalf("esc did not cancel selector: mode=%v pending=%T focused=%v", m.interactionMode, m.pending, m.inputFocused)
			}
		})
	}
}

func TestRendererKeepsPendingGenericAndInline(t *testing.T) {
	m := submitText(t, workspaceChat(t), "cable 10")
	plain := ansi.Strip(m.View().Content)
	for _, forbidden := range []string{"missing field", "question_request", "fixture", "confidence", "state", "Seleccioná una opción", "wizard"} {
		if strings.Contains(strings.ToLower(plain), strings.ToLower(forbidden)) {
			t.Fatalf("renderer leaked %q: %q", forbidden, plain)
		}
	}
	if strings.Count(plain, "¿Qué aislamiento necesitas?") != 1 || strings.Count(plain, "THW-LS") != 1 {
		t.Fatalf("pending was duplicated or omitted: %q", plain)
	}
}

func TestQuestionViewportOwnsPendingAndHistoryScroll(t *testing.T) {
	m := workspaceChat(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	for i := 0; i < 30; i++ {
		m.appendMessage(conversationMessage{role: roleGARFEX, kind: kindText, text: "historial suficientemente largo para desplazarse"})
	}
	m.input = "draft que debe sobrevivir"
	m.setPendingQuestion("¿Qué opción?", []string{"Uno", "Dos", "Tres", "Cuatro"})
	content := m.viewport.GetContent()
	for _, visible := range []string{"¿Qué opción?", "Uno", "Dos", "Tres", "Cuatro"} {
		if !strings.Contains(content, visible) {
			t.Fatalf("pending missing from viewport content %q: %q", visible, content)
		}
	}
	plain := ansi.Strip(m.View().Content)
	for _, visible := range []string{"¿Qué opción?", "Uno", "Dos", "Tres", "Cuatro", "GARFEX / MATERIALES MAESTROS", "esc cancelar"} {
		if !strings.Contains(plain, visible) {
			t.Fatalf("dock or fixed chrome missing %q: %q", visible, plain)
		}
	}
	selection, offset := m.choiceIndex, m.viewport.YOffset()
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	if m.choiceIndex != selection || m.viewport.YOffset() >= offset || m.pending == nil {
		t.Fatalf("page up changed pending state: selection=%d offset=%d pending=%T", m.choiceIndex, m.viewport.YOffset(), m.pending)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.choiceIndex != selection+1 || m.input != "draft que debe sobrevivir" || m.viewport.YOffset() >= offset {
		t.Fatalf("down changed history or draft: selection=%d offset=%d draft=%q", m.choiceIndex, m.viewport.YOffset(), m.input)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}))
	if !m.viewport.AtTop() || m.choiceIndex != selection+1 || m.pending == nil {
		t.Fatal("home did not move only the viewport")
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	if !m.viewport.AtBottom() || m.choiceIndex != selection+1 || m.pending == nil {
		t.Fatal("end did not return only the viewport to the bottom")
	}
}

func TestQuestionResizeKeepsDockSelectionAndDraft(t *testing.T) {
	m := workspaceChat(t)
	m.input = "draft persistente"
	m.setPendingQuestion("¿Qué opción?", []string{"Uno", "Dos", "Tres"})
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	selection := m.choiceIndex
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 10})
	if m.choiceIndex != selection || m.input != "draft persistente" {
		t.Fatalf("small resize lost selection or draft: selection=%d draft=%q", m.choiceIndex, m.input)
	}
	smallHeight := m.viewport.Height()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.viewport.Height() <= smallHeight || m.choiceIndex != selection || m.input != "draft persistente" {
		t.Fatalf("large resize did not expand viewport or preserve state: height=%d selection=%d draft=%q", m.viewport.Height(), m.choiceIndex, m.input)
	}
}

func TestActiveInteractionHighlightsSelectedOptionForEveryPendingType(t *testing.T) {
	tests := []struct {
		name    string
		pending InteractionMessage
		labels  []string
	}{
		{name: "question", pending: QuestionRequest{Question: "Pick", Options: []Option{{Label: "One"}, {Label: "Two"}}}, labels: []string{"One", "Two"}},
		{name: "confirmation", pending: ConfirmationRequest{Question: "Continue?", ConfirmLabel: "Confirm", CancelLabel: "Cancel"}, labels: []string{"Confirm", "Cancel"}},
		{name: "action", pending: ActionRequest{Question: "Choose", Actions: []Action{{Label: "Run"}, {Label: "Stop"}}}, labels: []string{"Run", "Stop"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := renderActiveInteraction(tt.pending, 0)
			second := renderActiveInteraction(tt.pending, 1)
			plain := ansi.Strip(first)
			if !strings.Contains(plain, "❯ "+tt.labels[0]) || !strings.Contains(plain, "  "+tt.labels[1]) {
				t.Fatalf("plain selection = %q", plain)
			}
			if strings.Contains(plain, "❯ "+tt.labels[1]) {
				t.Fatalf("unselected option has active marker = %q", plain)
			}
			if first == ansi.Strip(first) || second == ansi.Strip(second) {
				t.Fatalf("selection has no visual styling: first=%q second=%q", first, second)
			}
			if first == second {
				t.Fatalf("changing selected index did not change rendered output")
			}
		})
	}
}

func TestSelectionNavigationRebuildsVisibleHighlight(t *testing.T) {
	m := workspaceChat(t)
	m.setPendingQuestion("Pick", []string{"One", "Two", "Three"})
	first := ansi.Strip(m.View().Content)
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	second := ansi.Strip(m.View().Content)
	if m.choiceIndex != 1 || !strings.Contains(first, "❯ One") || !strings.Contains(second, "❯ Two") || strings.Contains(second, "❯ One") {
		t.Fatalf("down highlight = index %d, view %q", m.choiceIndex, second)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	third := ansi.Strip(m.View().Content)
	if m.choiceIndex != 0 || !strings.Contains(third, "❯ One") || strings.Contains(third, "❯ Two") {
		t.Fatalf("up highlight = index %d, view %q", m.choiceIndex, third)
	}
}

func TestSelectionResizeAndPagingPreserveHighlightState(t *testing.T) {
	m := workspaceChat(t)
	m.setPendingQuestion("Pick", []string{"One", "Two", "Three"})
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	selection := m.choiceIndex
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 10})
	if m.choiceIndex != selection || !strings.Contains(ansi.Strip(m.viewport.GetContent()), "❯ Two") {
		t.Fatalf("resize changed selection or highlight: index=%d", m.choiceIndex)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	if m.choiceIndex != selection || !strings.Contains(ansi.Strip(m.viewport.GetContent()), "❯ Two") {
		t.Fatalf("paging changed selection or highlight: index=%d", m.choiceIndex)
	}
}

func searchableModel(agent InteractionAgent) Model {
	m := NewWithAgent(Handlers{}, agent)
	m.screen = screenWorkspace
	m.pending = QuestionRequest{
		Key:           "material",
		Prompt:        "Choose a material",
		SelectionMode: SelectionSearchable,
		Options: []Option{
			{Label: "THW 10 Negro", Value: "black-10"},
			{Label: "THW 12 Blanco", Value: "white-12"},
			{Label: "XHHW 10 Negro", Value: "xhhw-black-10"},
		},
	}
	m.interactionMode = interactionModeSearchable
	m.syncChoiceFields()
	m.resizeViewport()
	return m
}

func TestSearchableSelectFiltering(t *testing.T) {
	options := []Option{{Label: "THW 10 Negro"}, {Label: "THW 12 Blanco"}, {Label: "XHHW 10 Negro"}}
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "empty query", want: []string{"THW 10 Negro", "THW 12 Blanco", "XHHW 10 Negro"}},
		{name: "case insensitive", query: "ThW", want: []string{"THW 10 Negro", "THW 12 Blanco"}},
		{name: "tokens are AND and order independent", query: "negro 10 thw", want: []string{"THW 10 Negro"}},
		{name: "substring", query: "hhw", want: []string{"XHHW 10 Negro"}},
		{name: "no matches", query: "copper", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterOptions(options, tt.query)
			if len(got) != len(tt.want) {
				t.Fatalf("matches = %#v, want %#v", got, tt.want)
			}
			for i, option := range got {
				if option.Label != tt.want[i] {
					t.Fatalf("match[%d] = %q, want %q", i, option.Label, tt.want[i])
				}
			}
		})
	}
}

func TestSearchableSelectJKNavigateWithPrintableText(t *testing.T) {
	tests := []struct {
		name string
		key  rune
		from int
		want int
	}{
		{name: "j moves down", key: 'j', from: 0, want: 1},
		{name: "k moves up", key: 'k', from: 1, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := searchableModel(&recordingAgent{})
			m.choiceIndex = tt.from
			m, _ = update(t, m, key(tt.key))
			if m.choiceIndex != tt.want || m.searchQuery != "" {
				t.Fatalf("state after %q = index %d query %q, want index %d and empty query", tt.key, m.choiceIndex, m.searchQuery, tt.want)
			}
		})
	}
}

func TestSearchableSelectUpdatesLocallyAndClampsCursor(t *testing.T) {
	m := searchableModel(&recordingAgent{})
	m.choiceIndex = 2
	m, _ = update(t, m, key('n'))
	m, _ = update(t, m, key('e'))
	m, _ = update(t, m, key('g'))
	m, _ = update(t, m, key('r'))
	m, _ = update(t, m, key('o'))
	if m.searchQuery != "negro" || m.choiceIndex != 1 || len(m.pendingOptions()) != 2 {
		t.Fatalf("filtered state = query %q index %d options %#v", m.searchQuery, m.choiceIndex, m.pendingOptions())
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	if m.searchQuery != "negr" {
		t.Fatalf("backspace query = %q", m.searchQuery)
	}
}

func TestSearchableSelectEnterSendsVisibleValueInOrder(t *testing.T) {
	agent := &recordingAgent{}
	m := searchableModel(agent)
	for _, char := range []rune("negro") {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m, _ = update(t, m, enter())
	if agent.last.Kind != InputSelection || agent.last.Key != "material" || agent.last.Value != "xhhw-black-10" || len(agent.last.Values) != 0 {
		t.Fatalf("selection payload = %#v", agent.last)
	}
}

func TestSearchableSelectDoesNotConfirmNoMatchAndCancels(t *testing.T) {
	agent := &recordingAgent{}
	m := searchableModel(agent)
	for _, char := range []rune("copper") {
		m, _ = update(t, m, key(char))
	}
	m, _ = update(t, m, enter())
	if agent.calls != 0 || m.pending == nil {
		t.Fatalf("no-match enter changed state: calls=%d pending=%T", agent.calls, m.pending)
	}
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if agent.last.Kind != InputCancel || m.pending != nil || m.interactionMode != interactionModeChat || m.searchQuery != "" {
		t.Fatalf("cancel state = input %#v pending %T mode %v query %q", agent.last, m.pending, m.interactionMode, m.searchQuery)
	}
}

func TestSearchableSelectRendersPromptQueryCountAndFocus(t *testing.T) {
	m := searchableModel(&recordingAgent{})
	for _, char := range []rune("10") {
		m, _ = update(t, m, key(char))
	}
	plain := ansi.Strip(m.viewport.GetContent())
	for _, want := range []string{"Choose a material", "Buscar: 10", "Coincidencias: 2", "❯ THW 10 Negro", "XHHW 10 Negro"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("rendered searchable select missing %q: %q", want, plain)
		}
	}
}

func TestQuestionSelectionDoesNotUsePromptHistory(t *testing.T) {
	m := workspaceChat(t)
	m.promptHistory = []string{"previous prompt"}
	m.input = "draft"
	m.setPendingQuestion("Pick", []string{"One", "Two"})
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.choiceIndex != 1 || m.promptHistoryCursor != -1 || m.input != "draft" {
		t.Fatalf("question selection interfered with prompt history: index=%d cursor=%d input=%q", m.choiceIndex, m.promptHistoryCursor, m.input)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func historyContains(values []conversationMessage, want string) bool {
	for _, value := range values {
		if strings.Contains(value.text, want) {
			return true
		}
	}
	return false
}
