package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func key(code rune) tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Code: code, Text: string(code)}) }
func enter() tea.KeyPressMsg        { return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}) }
func space() tea.KeyPressMsg        { return tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}) }
func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func TestModelMenuAndCommands(t *testing.T) {
	called := false
	m := New(Handlers{Version: func() tea.Cmd { called = true; return func() tea.Msg { return resultMsg{text: "ok"} } }, Config: Status(), Status: Status()})
	if got := m.View().Content; !strings.Contains(got, "> Version") || !strings.Contains(got, "Config check\n") || !strings.Contains(got, "GARFEX status\n") || !strings.Contains(got, "Exit") {
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
	_, quit := update(t, m, enter())
	if quit == nil || called {
		t.Fatal("exit must quit before handler")
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
	if m.screen != screenResult || strings.Contains(m.View().Content, "\x1b") {
		t.Fatal("success must be sanitized")
	}
	m, _ = update(t, m, key('b'))
	if m.screen != screenMenu || m.cursor != 2 {
		t.Fatal("back must preserve cursor")
	}
	m, _ = update(t, m, resultMsg{err: errors.New("bad\x7f")})
	if m.screen != screenError || strings.Contains(m.View().Content, "\x7f") {
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
	if len(m.items) != 4 || m.items[0].Label != "Version" || m.items[1].Label != "Config check" || m.items[2].Label != "GARFEX status" || m.items[3].Label != "Exit" {
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
		s    screen
		want string
	}{{screenMenu, "GARFEX\n\n> Version\n  Config check\n  GARFEX status\n  Exit\n\nUse arrows or j/k, enter or space to select."}, {screenLoading, "Loading Version..."}, {screenResult, "ok\n\nPress b or esc to return."}, {screenError, "bad\n\nPress r to retry, b or esc to return."}, {screenMinSize, "Terminal must be at least 40x10."}} {
		m.screen, m.result = tt.s, "bad"
		if tt.s == screenResult {
			m.result = "ok"
		}
		if one, two := m.View().Content, m.View().Content; one != two || one != tt.want {
			t.Fatalf("view = %q, want %q", one, tt.want)
		}
	}
}
