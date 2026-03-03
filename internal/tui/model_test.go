package tui

import (
	"os"
	"path/filepath"
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
	m := NewModelWithTargetRoot(records, t.TempDir())
	if m.CurrentSourceFolder() != "/work" {
		t.Fatalf("unexpected first source folder: %q", m.CurrentSourceFolder())
	}
}

func TestSessionsForCurrentSourceFiltersToSourceFolder(t *testing.T) {
	root := t.TempDir()
	records := []sessionindex.SessionRecord{
		{SessionID: "c", SessionMetaCWD: "/repo/other"},
		{SessionID: "a", SessionMetaCWD: "/repo/src"},
		{SessionID: "b", SessionMetaCWD: "/repo/src/sub"},
	}
	m := NewModelWithTargetRoot(records, root)
	for i, n := range m.sourceNodes {
		if n.Path == "/repo/src" {
			m.sourcePane.cursor = i
		}
	}
	rows := m.SessionsForCurrentSource()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for current source, got %d", len(rows))
	}
}

func TestTargetPaneUsesHostFilesystemTree(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "projects", "app1"))
	mustMkdirAll(t, filepath.Join(root, "projects", "app2"))
	mustMkdirAll(t, filepath.Join(root, ".hidden"))
	m := NewModelWithTargetRoot(nil, root)
	paths := targetPaths(m)
	if !containsPath(paths, filepath.Join(root, "projects")) {
		t.Fatalf("expected target tree to include projects dir, got %#v", paths)
	}
	if containsPath(paths, filepath.Join(root, ".hidden")) {
		t.Fatalf("expected hidden dir to be excluded, got %#v", paths)
	}
	if m.CurrentTargetFolder() != root {
		t.Fatalf("expected initial target folder %q, got %q", root, m.CurrentTargetFolder())
	}
}

func TestSourcePaneCompactsSingleChildChainsWithoutDirectHistory(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "1", SessionMetaCWD: "/home/user/dev/project"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())
	if len(m.sourceNodes) != 1 {
		t.Fatalf("expected one compacted source node, got %#v", m.sourceNodes)
	}
	if got := m.sourceNodes[0].Name; got != "home/user/dev/project" {
		t.Fatalf("expected compacted label, got %q", got)
	}
	if got := m.sourceNodes[0].Path; got != "/home/user/dev/project" {
		t.Fatalf("expected compacted node to keep leaf path, got %q", got)
	}
	if got := m.CurrentSourceFolder(); got != "/home/user/dev/project" {
		t.Fatalf("expected current source to resolve to leaf path, got %q", got)
	}
}

func TestFitPathLabelUsesMiddleEllipsisOnlyWhenNeeded(t *testing.T) {
	if got := fitPathLabel("home/user/dev/project", 64); got != "home/user/dev/project" {
		t.Fatalf("expected full label when it fits, got %q", got)
	}
	if got := fitPathLabel("home/user/dev/project", 16); got != "home/.../project" {
		t.Fatalf("expected middle ellipsis form for narrow width, got %q", got)
	}
}

func TestViewContainsPaneHeadersAndKeys(t *testing.T) {
	m := NewModelWithTargetRoot(nil, t.TempDir())
	view := m.View()
	for _, expected := range []string{"Source", "Target", "Sessions", "F5", "refresh", "F6", "move", "Status:"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q: %q", expected, view)
		}
	}
}

func TestUpdateQuitKeyReturnsQuitCmd(t *testing.T) {
	m := NewModelWithTargetRoot(nil, t.TempDir())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command on q key")
	}
}

func TestWindowSizeUsesFullWidth(t *testing.T) {
	m := NewModelWithTargetRoot([]sessionindex.SessionRecord{{SessionID: "1", SessionMetaCWD: "/work/a"}}, t.TempDir())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	mm := updated.(Model)
	view := mm.View()
	lines := strings.Split(view, "\n")
	if len(lines) == 0 || len([]rune(lines[0])) < 80 {
		t.Fatalf("expected wide first row after resize, got %q", lines[0])
	}
}

func TestWindowSizeRespectsTotalHeight(t *testing.T) {
	m := NewModelWithTargetRoot([]sessionindex.SessionRecord{{SessionID: "1", SessionMetaCWD: "/work/a"}}, t.TempDir())
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
	m := NewModelWithTargetRoot(records, t.TempDir())
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

func TestTabCyclesSourceSessionsTargetAndScrollsIndependently(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "dst1"))
	records := make([]sessionindex.SessionRecord, 0, 12)
	for i := 0; i < 12; i++ {
		records = append(records, sessionindex.SessionRecord{SessionID: string(rune('a' + i)), SessionMetaCWD: "/repo/src/sub"})
	}
	m := NewModelWithTargetRoot(records, root)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 18})
	mm := updated.(Model)
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
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm = updated.(Model)
	if mm.activePanel != panelTarget {
		t.Fatalf("expected target panel active after second tab, got %d", mm.activePanel)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	mm = updated.(Model)
	if mm.activePanel != panelSessions {
		t.Fatalf("expected shift+tab to return to sessions, got %d", mm.activePanel)
	}
}

func TestSpaceTogglesCurrentSessionSelection(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "a", SessionFile: "/tmp/a.jsonl", SessionMetaCWD: "/repo/src"},
		{SessionID: "b", SessionFile: "/tmp/b.jsonl", SessionMetaCWD: "/repo/src"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(Model)

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

func TestLeftRightJumpBetweenSourceAndTarget(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "dst1"))
	m := NewModelWithTargetRoot([]sessionindex.SessionRecord{{SessionID: "a", SessionMetaCWD: "/repo/src"}}, root)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(Model)
	if mm.activePanel != panelTarget {
		t.Fatalf("expected right to jump to target, got %d", mm.activePanel)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	mm = updated.(Model)
	if mm.activePanel != panelSource {
		t.Fatalf("expected left from target root to jump to source, got %d", mm.activePanel)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm = updated.(Model)
	if mm.activePanel != panelSessions {
		t.Fatalf("expected tab to move to sessions, got %d", mm.activePanel)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm = updated.(Model)
	if mm.activePanel != panelTarget {
		t.Fatalf("expected right from sessions to jump to target, got %d", mm.activePanel)
	}
}

func TestLeftInTargetCollapsesOrAscendsBeforeSwitchingSource(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "projects", "app1"))
	m := NewModelWithTargetRoot(nil, root)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm = updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm = updated.(Model)
	if mm.CurrentTargetFolder() != filepath.Join(root, "projects") {
		t.Fatalf("expected projects selected, got %q", mm.CurrentTargetFolder())
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	mm = updated.(Model)
	if mm.activePanel != panelTarget {
		t.Fatalf("expected left in target subtree to stay in target, got %d", mm.activePanel)
	}
	if mm.CurrentTargetFolder() != root {
		t.Fatalf("expected left to move to parent/root, got %q", mm.CurrentTargetFolder())
	}
}

func TestTargetRightExpandsAndLeftCollapses(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "projects", "app1"))
	m := NewModelWithTargetRoot(nil, root)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(Model)
	if mm.activePanel != panelTarget {
		t.Fatalf("expected target active, got %d", mm.activePanel)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm = updated.(Model)
	if len(mm.targetNodes) < 2 {
		t.Fatalf("expected expanded target tree, got %#v", mm.targetNodes)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm = updated.(Model)
	if mm.CurrentTargetFolder() != filepath.Join(root, "projects") {
		t.Fatalf("expected projects selected, got %q", mm.CurrentTargetFolder())
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm = updated.(Model)
	if !containsPath(targetPaths(mm), filepath.Join(root, "projects", "app1")) {
		t.Fatalf("expected app1 visible after expand, got %#v", targetPaths(mm))
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	mm = updated.(Model)
	if containsPath(targetPaths(mm), filepath.Join(root, "projects", "app1")) {
		t.Fatalf("expected app1 hidden after collapse, got %#v", targetPaths(mm))
	}
}

func TestSelectAllOnlySelectsCurrentSourceSessions(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "a", SessionFile: "/tmp/a.jsonl", SessionMetaCWD: "/repo/other"},
		{SessionID: "b", SessionFile: "/tmp/b.jsonl", SessionMetaCWD: "/repo/src"},
		{SessionID: "c", SessionFile: "/tmp/c.jsonl", SessionMetaCWD: "/repo/src/sub"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())
	for i, n := range m.sourceNodes {
		if n.Path == "/repo/src" {
			m.sourcePane.cursor = i
		}
	}
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
	m := NewModelWithTargetRoot([]sessionindex.SessionRecord{{SessionID: "a", SessionFile: "/tmp/a.jsonl", SessionMetaCWD: "/repo/src"}}, t.TempDir())
	m.selected["/tmp/a.jsonl"] = struct{}{}
	view := m.View()
	if !strings.Contains(view, "[*] a") {
		t.Fatalf("expected selected marker in view, got %q", view)
	}
	if !strings.Contains(view, "Selected: 1") {
		t.Fatalf("expected selected count in status, got %q", view)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func targetPaths(m Model) []string {
	out := make([]string, 0, len(m.targetNodes))
	for _, n := range m.targetNodes {
		out = append(out, n.Path)
	}
	return out
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}
