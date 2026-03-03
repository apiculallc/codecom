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
	m.sourcePane.cursor = 1
	rows := m.SessionsForCurrentSource()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for current source, got %d", len(rows))
	}
}

func TestViewContainsPaneHeadersAndKeys(t *testing.T) {
	m := NewModel(nil)
	view := m.View()
	for _, expected := range []string{"Source", "Target", "Sessions", "F5", "refresh", "F6", "move", "Status:"} {
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

func TestWindowSizeUsesFullWidth(t *testing.T) {
	m := NewModel([]sessionindex.SessionRecord{{SessionID: "1", SessionMetaCWD: "/work/a"}})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	mm := updated.(Model)
	view := mm.View()
	lines := strings.Split(view, "\n")
	if len(lines) == 0 || len([]rune(lines[0])) < 80 {
		t.Fatalf("expected wide first row after resize, got %q", lines[0])
	}
}

func TestWindowSizeRespectsTotalHeight(t *testing.T) {
	m := NewModel([]sessionindex.SessionRecord{{SessionID: "1", SessionMetaCWD: "/work/a"}})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	mm := updated.(Model)
	view := strings.TrimRight(mm.View(), "\n")
	lines := strings.Split(view, "\n")
	if len(lines) > 24 {
		t.Fatalf("expected rendered height <= 24, got %d", len(lines))
	}
}

func TestSourcePaneScrolls(t *testing.T) {
	records := make([]sessionindex.SessionRecord, 0, 20)
	for i := 0; i < 20; i++ {
		records = append(records, sessionindex.SessionRecord{SessionID: string(rune('a' + i)), SessionMetaCWD: "/repo/" + string(rune('a'+i))})
	}
	m := NewModel(records)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 16})
	mm := updated.(Model)
	for i := 0; i < 8; i++ {
		updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyDown})
		mm = updated.(Model)
	}
	if mm.sourcePane.offset == 0 {
		t.Fatal("expected source pane to scroll")
	}
	if mm.sourcePane.cursor >= mm.sourcePane.offset+mm.topViewportHeight() {
		t.Fatalf("cursor should remain visible inside pane body: cursor=%d offset=%d body=%d", mm.sourcePane.cursor, mm.sourcePane.offset, mm.topViewportHeight())
	}
}

func TestTabMovesToSessionPaneAndScrollsIndependently(t *testing.T) {
	records := make([]sessionindex.SessionRecord, 0, 12)
	for i := 0; i < 12; i++ {
		records = append(records, sessionindex.SessionRecord{SessionID: string(rune('a' + i)), SessionMetaCWD: "/repo/src/sub"})
	}
	m := NewModel(records)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm = updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm = updated.(Model)
	if mm.activePanel != panelSessions {
		t.Fatalf("expected session panel active, got %d", mm.activePanel)
	}
	for i := 0; i < 6; i++ {
		updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyDown})
		mm = updated.(Model)
	}
	if mm.sessionPane.cursor == 0 {
		t.Fatal("expected session pane cursor to move")
	}
}

func TestSpaceTogglesCurrentSessionSelection(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "a", SessionFile: "/tmp/a.jsonl", SessionMetaCWD: "/repo/src"},
		{SessionID: "b", SessionFile: "/tmp/b.jsonl", SessionMetaCWD: "/repo/src"},
	}
	m := NewModel(records)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm = updated.(Model)

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = updated.(Model)
	if mm.SelectedCount() != 1 {
		t.Fatalf("expected 1 selected session, got %d", mm.SelectedCount())
	}
	if _, ok := mm.selected["/tmp/a.jsonl"]; !ok {
		t.Fatal("expected current session to be selected")
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeySpace})
	mm = updated.(Model)
	if mm.SelectedCount() != 0 {
		t.Fatalf("expected selection to toggle off, got %d", mm.SelectedCount())
	}
}

func TestSelectAllOnlySelectsCurrentSourceSessions(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "a", SessionFile: "/tmp/a.jsonl", SessionMetaCWD: "/repo/other"},
		{SessionID: "b", SessionFile: "/tmp/b.jsonl", SessionMetaCWD: "/repo/src"},
		{SessionID: "c", SessionFile: "/tmp/c.jsonl", SessionMetaCWD: "/repo/src/sub"},
	}
	m := NewModel(records)
	m.sourcePane.cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	mm := updated.(Model)
	if mm.SelectedCount() != 2 {
		t.Fatalf("expected 2 selected sessions, got %d", mm.SelectedCount())
	}
	if _, ok := mm.selected["/tmp/a.jsonl"]; ok {
		t.Fatal("expected select-all to ignore sessions outside current source")
	}
}

func TestViewMarksSelectedSessions(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "a", SessionFile: "/tmp/a.jsonl", SessionMetaCWD: "/repo/src"},
	}
	m := NewModel(records)
	m.selected["/tmp/a.jsonl"] = struct{}{}
	view := m.View()
	if !strings.Contains(view, "[*] a") {
		t.Fatalf("expected selected marker in view, got %q", view)
	}
	if !strings.Contains(view, "Selected: 1") {
		t.Fatalf("expected selected count in status, got %q", view)
	}
}
