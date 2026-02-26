package tui

import (
	"strings"
	"testing"

	"codecom/internal/sessionindex"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelAndCurrentSource(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "1", SessionMetaCWD: "/work/a"},
		{SessionID: "2", SessionMetaCWD: "/work/b"},
	}
	m := NewModel(records)
	if m.CurrentSourceFolder() != "/work/a" {
		t.Fatalf("unexpected first source folder: %q", m.CurrentSourceFolder())
	}
}

func TestSessionsForCurrentSourceFiltersToSourceFolder(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "c", SessionMetaCWD: "/repo/other"},
		{SessionID: "a", SessionMetaCWD: "/repo/src"},
		{SessionID: "b", SessionMetaCWD: "/repo/src/sub"},
	}
	m := NewModel(records)
	m.sourceIndex = 1 // /repo/src
	rows := m.SessionsForCurrentSource()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for current source, got %d", len(rows))
	}
}

func TestViewContainsPaneHeadersAndKeys(t *testing.T) {
	m := NewModel(nil)
	view := m.View()
	for _, expected := range []string{"Source", "Target", "Sessions", "F5 refresh", "F6 move"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q: %q", expected, view)
		}
	}
}

func TestUpdateQuitKeyReturnsQuitCmd(t *testing.T) {
	m := NewModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command on q key")
	}
}
