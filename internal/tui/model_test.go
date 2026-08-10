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
func space() tea.KeyPressMsg         { return tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}) }
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
	m := New(Handlers{Version: func() tea.Cmd { called = true; return func() tea.Msg { return resultMsg{text: "ok"} } }, Config: Status(), Status: Status()})
	if got := ansi.Strip(m.View().Content); containsFullWordmark(got) || !strings.Contains(got, "GARFEX") || !strings.Contains(normalized(got), officialTagline) || !strings.Contains(got, "› 01  Versión") || !strings.Contains(got, "02  Verificar configuración") || !strings.Contains(got, "03  Estado de GARFEX") || !strings.Contains(got, "04  Salir") || !strings.Contains(got, "j/k navegar") || strings.Contains(got, "/\\/") || strings.Contains(got, "╭") {
		t.Fatalf("menu = %q", got)
	}
	for range 8 {
		m, _ = update(t, m, key('k'))
	}
	if m.cursor != 0 {
		t.Fatalf("upper cursor = %d", m.cursor)
	}
	for range 8 {
		m, _ = update(t, m, key('j'))
	}
	if m.cursor != 3 {
		t.Fatalf("lower cursor = %d", m.cursor)
	}
	m.items[m.cursor].Handler = func() tea.Cmd { called = true; return nil }
	_, quit := update(t, m, enter())
	if quit == nil || called {
		t.Fatal("localized exit must quit without calling a handler")
	}
	m, _ = update(t, New(Handlers{Version: func() tea.Cmd { called = true; return func() tea.Msg { return resultMsg{text: "ok"} } }, Config: Status(), Status: Status()}), enter())
	if m.screen != screenLoading || !called {
		t.Fatalf("loading=%v called=%v", m.screen, called)
	}
	_, cmd := update(t, New(Handlers{Version: Status(), Config: Status(), Status: Status()}), enter())
	if cmd == nil {
		t.Fatal("selection must return command")
	}
	_, cmd = update(t, New(Handlers{Version: Status(), Config: Status(), Status: Status()}), space())
	if cmd == nil {
		t.Fatal("space selection must return command")
	}
	_, quit = update(t, m, key('q'))
	if quit == nil {
		t.Fatal("q must quit")
	}
}

func TestModelResultsSizingAndView(t *testing.T) {
	m := New(Handlers{Version: Status(), Config: Status(), Status: Status()})
	m.cursor = 2
	m, _ = update(t, m, resultMsg{text: "ok\x1b"})
	if got := ansi.Strip(m.View().Content); m.screen != screenResult || strings.Contains(got, "\x1b") {
		t.Fatal("success must be sanitized")
	}
	m, _ = update(t, m, key('b'))
	if m.screen != screenMenu || m.cursor != 2 {
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
	if m.screen != screenMenu {
		t.Fatal("valid resize must recover menu")
	}
	if one, two := m.View().Content, m.View().Content; one != two {
		t.Fatal("view must be deterministic")
	}
}

func TestModelMissingContracts(t *testing.T) {
	m := New(Handlers{Version: Status(), Config: Status(), Status: Status()})
	if len(m.items) != 4 || m.items[0].Label != "Versión" || m.items[1].Label != "Verificar configuración" || m.items[2].Label != "Estado de GARFEX" || m.items[3].Label != "Salir" || !m.items[3].Quit {
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
	m, cmd := update(t, m, key('r'))
	if m.screen != screenLoading || cmd == nil {
		t.Fatal("error retry must load and return a command")
	}
	if m, cmd = update(t, m, key('r')); m.screen != screenLoading || cmd != nil {
		t.Fatal("loading must ignore retry")
	}
	m, _ = update(t, New(Handlers{Version: Status(), Config: Status(), Status: Status()}), resultMsg{text: "ok"})
	m, _ = update(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if m.screen != screenMenu {
		t.Fatal("esc must return to menu")
	}
	for _, tt := range []struct {
		name string
		s    screen
		want []string
	}{
		{"menu", screenMenu, []string{"MENÚ PRINCIPAL", "› 01  Versión", "04  Salir", "j/k navegar", "enter elegir", "q salir"}},
		{"loading", screenLoading, []string{"PROCESANDO", "Procesando Versión...", "Espere un momento", "q salir"}},
		{"success", screenResult, []string{"OPERACIÓN COMPLETADA", "ok", "b/esc volver", "q salir"}},
		{"error", screenError, []string{"ERROR DE OPERACIÓN", "bad", "r reintentar", "b/esc volver"}},
		{"minimum", screenMinSize, []string{"La terminal debe tener al menos 40x10."}},
	} {
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
		{"exact full height", 102, 21, true},
		{"insufficient full height", 102, 20, false},
		{"ordinary terminal", 80, 24, false},
		{"minimum boundary", 40, 10, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Handlers{Version: Status(), Config: Status(), Status: Status()})
			m, _ = update(t, m, tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			got := m.View().Content
			if lipgloss.Width(got) != tt.width || lipgloss.Height(got) != tt.height {
				t.Fatalf("view size = %dx%d, want %dx%d", lipgloss.Width(got), lipgloss.Height(got), tt.width, tt.height)
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
