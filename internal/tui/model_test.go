package tui

import (
	"fmt"
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
	for _, expected := range []string{"Source", "Target", "Sessions", "F5", "refresh", "F6", "move", "/", "filter", "Enter", "open", "Status:"} {
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

func TestSlashEntersSourceFilterModeAndFuzzyMatches(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "a", SessionMetaCWD: "/repo/project-alpha"},
		{SessionID: "b", SessionMetaCWD: "/repo/project-beta"},
		{SessionID: "c", SessionMetaCWD: "/repo/docs"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm := updated.(Model)
	if !mm.filterMode {
		t.Fatal("expected filter mode after slash")
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p', 'h', 'a'}})
	mm = updated.(Model)
	if mm.sourceFilter != "pha" {
		t.Fatalf("expected source filter to be updated, got %q", mm.sourceFilter)
	}
	if got := len(mm.visibleSourceNodes()); got != 1 {
		t.Fatalf("expected 1 fuzzy-matched source node, got %d", got)
	}
	if got := mm.CurrentSourceFolder(); got != "/repo/project-alpha" {
		t.Fatalf("expected filtered source selection to track visible row, got %q", got)
	}
}

func TestSlashEntersTargetFilterModeAndFuzzyMatches(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "home-root")
	mustMkdirAll(t, root)
	mustMkdirAll(t, filepath.Join(root, "alpha"))
	mustMkdirAll(t, filepath.Join(root, "beta"))
	mustMkdirAll(t, filepath.Join(root, "docs"))
	m := NewModelWithTargetRoot(nil, root)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(Model)
	if mm.activePanel != panelTarget {
		t.Fatalf("expected target active, got %d", mm.activePanel)
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm = updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b', 't', 'a'}})
	mm = updated.(Model)
	if mm.targetFilter != "bta" {
		t.Fatalf("expected target filter to be updated, got %q", mm.targetFilter)
	}
	if got := len(mm.visibleTargetNodes()); got != 1 {
		t.Fatalf("expected 1 fuzzy-matched target node, got %d", got)
	}
	if got := mm.CurrentTargetFolder(); got != filepath.Join(root, "beta") {
		t.Fatalf("expected filtered target selection to track visible row, got %q", got)
	}
}

func TestFilterEscClearsCurrentPaneQuery(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "a", SessionMetaCWD: "/repo/project-alpha"},
		{SessionID: "b", SessionMetaCWD: "/repo/docs"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p', 'h', 'a'}})
	mm = updated.(Model)
	if len(mm.visibleSourceNodes()) != 1 {
		t.Fatalf("expected filtered source nodes before clear, got %d", len(mm.visibleSourceNodes()))
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm = updated.(Model)
	if mm.filterMode {
		t.Fatal("expected filter mode to end on esc")
	}
	if mm.sourceFilter != "" {
		t.Fatalf("expected source filter to be cleared, got %q", mm.sourceFilter)
	}
	if len(mm.visibleSourceNodes()) != len(mm.sourceNodes) {
		t.Fatalf("expected source nodes to be restored after esc, got %d vs %d", len(mm.visibleSourceNodes()), len(mm.sourceNodes))
	}
}

func TestFilterIgnoredInSessionsPane(t *testing.T) {
	m := NewModelWithTargetRoot([]sessionindex.SessionRecord{{SessionID: "a", SessionMetaCWD: "/repo/src"}}, t.TempDir())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(Model)
	if mm.activePanel != panelSessions {
		t.Fatalf("expected sessions active, got %d", mm.activePanel)
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm = updated.(Model)
	if mm.filterMode {
		t.Fatal("expected filter mode to stay off in sessions pane")
	}
	if mm.status != "filter only works in source and target panes" {
		t.Fatalf("unexpected status: %q", mm.status)
	}
}

func TestEnterOnFilteredSourceScopesTreeAndAddsParentRow(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "a", SessionMetaCWD: "/repo/project-alpha"},
		{SessionID: "b", SessionMetaCWD: "/repo/project-beta"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'l', 'p'}})
	mm = updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)

	if mm.filterMode {
		t.Fatal("expected filter mode to end after enter")
	}
	if mm.sourceFilter != "" {
		t.Fatalf("expected source filter to clear after enter, got %q", mm.sourceFilter)
	}
	nodes := mm.visibleSourceNodes()
	if len(nodes) < 2 {
		t.Fatalf("expected scoped source nodes with parent row, got %#v", nodes)
	}
	if !nodes[0].ParentNav || nodes[0].Name != ".." {
		t.Fatalf("expected parent row at top, got %#v", nodes[0])
	}
	if got := mm.CurrentSourceFolder(); got != "/repo/project-alpha" {
		t.Fatalf("expected current source to be entered node, got %q", got)
	}
}

func TestEnterOnFilteredTargetScopesTreeAndParentRowReturnsUp(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "home-root")
	mustMkdirAll(t, filepath.Join(root, "alpha", "sub"))
	mustMkdirAll(t, filepath.Join(root, "beta"))
	m := NewModelWithTargetRoot(nil, root)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm = updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'l', 'p'}})
	mm = updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)

	nodes := mm.visibleTargetNodes()
	if len(nodes) < 2 {
		t.Fatalf("expected scoped target nodes with parent row, got %#v", nodes)
	}
	if !nodes[0].ParentNav || nodes[0].Name != ".." {
		t.Fatalf("expected parent row at top, got %#v", nodes[0])
	}
	if got := mm.CurrentTargetFolder(); got != filepath.Join(root, "alpha") {
		t.Fatalf("expected current target to be entered node, got %q", got)
	}

	mm.targetPane.cursor = 0
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if got := mm.CurrentTargetFolder(); got != filepath.Join(root, "alpha") {
		t.Fatalf("expected enter on parent row to preselect previous folder, got %q", got)
	}
}

func TestEnterTargetShowsChildrenImmediately(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "projects", "app1"))
	m := NewModelWithTargetRoot(nil, root)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm = updated.(Model)
	if got := mm.CurrentTargetFolder(); got != filepath.Join(root, "projects") {
		t.Fatalf("expected projects selected, got %q", got)
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if !containsPath(targetPaths(mm), filepath.Join(root, "projects", "app1")) {
		t.Fatalf("expected child folder visible after enter, got %#v", targetPaths(mm))
	}
}

func TestEnterParentPreselectsPreviousTargetFolder(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "projects", "app1"))
	mustMkdirAll(t, filepath.Join(root, "docs"))
	m := NewModelWithTargetRoot(nil, root)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := updated.(Model)
	for i, node := range mm.visibleTargetNodes() {
		if node.Path == filepath.Join(root, "projects") {
			mm.targetPane.cursor = i
			break
		}
	}
	if got := mm.CurrentTargetFolder(); got != filepath.Join(root, "projects") {
		t.Fatalf("expected projects selected before enter, got %q", got)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)

	// In scoped view, ".." is at top; select it and enter to return to parent.
	mm.targetPane.cursor = 0
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if got := mm.CurrentTargetFolder(); got != filepath.Join(root, "projects") {
		t.Fatalf("expected previous folder preselected after parent enter, got %q", got)
	}
}

func TestEnterOnSessionOpensConversationPopupAndEscCloses(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.jsonl")
	content := strings.Join([]string{
		`{"timestamp":"2026-03-09T10:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello\nworld"}]}}`,
		`{"timestamp":"2026-03-09T10:01:00Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi there"}]}}`,
	}, "\n")
	if err := os.WriteFile(sessionFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	records := []sessionindex.SessionRecord{
		{SessionID: "s1", SessionFile: sessionFile, SessionMetaCWD: "/repo/src"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(Model)
	if mm.activePanel != panelSessions {
		t.Fatalf("expected sessions panel active, got %d", mm.activePanel)
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if !mm.popupOpen {
		t.Fatal("expected popup to open on enter in sessions pane")
	}
	if len(mm.popupOffsets) < 2 {
		t.Fatalf("expected popup offsets to include conversation entries, got %#v", mm.popupOffsets)
	}
	view := mm.View()
	if !strings.Contains(view, "Conversation") {
		t.Fatalf("expected popup view to contain title, got %q", view)
	}
	if !strings.Contains(view, "You: hello world") {
		t.Fatalf("expected normalized user message in popup, got %q", view)
	}

	updated, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm = updated.(Model)
	if cmd != nil {
		t.Fatal("expected esc to close popup, not quit app")
	}
	if mm.popupOpen {
		t.Fatal("expected popup closed on esc")
	}
}

func TestPopupScrollsWithDownKey(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.jsonl")
	lines := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf(`{"timestamp":"2026-03-09T10:%02d:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"line %d"}]}}`, i%60, i))
	}
	if err := os.WriteFile(sessionFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	records := []sessionindex.SessionRecord{
		{SessionID: "s1", SessionFile: sessionFile, SessionMetaCWD: "/repo/src"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if !mm.popupOpen {
		t.Fatal("expected popup to open")
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm = updated.(Model)
	if mm.popupPane.cursor == 0 {
		t.Fatal("expected popup cursor to move down")
	}
}

func TestSourceTreeNodesCarrySessionIDs(t *testing.T) {
	records := []sessionindex.SessionRecord{
		{SessionID: "sid-1", SessionMetaCWD: "/repo/src"},
		{SessionID: "sid-2", SessionMetaCWD: "/repo/src/sub"},
	}
	m := NewModelWithTargetRoot(records, t.TempDir())
	var found bool
	for _, node := range m.sourceNodes {
		if node.Path != "/repo/src" {
			continue
		}
		found = true
		if len(node.SessionIDs) != 2 {
			t.Fatalf("expected /repo/src to aggregate 2 session ids, got %#v", node.SessionIDs)
		}
	}
	if !found {
		t.Fatal("expected /repo/src source node")
	}
}

func TestTargetTreeNodesCarryExactPathSessionIDs(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "repo", "src"))
	records := []sessionindex.SessionRecord{
		{SessionID: "sid-1", SessionMetaCWD: filepath.Join(root, "repo", "src")},
	}
	m := NewModelWithTargetRoot(records, root)
	m.targetExpanded[filepath.Join(root, "repo")] = struct{}{}
	m.reloadTargetNodes("")
	var found bool
	for _, node := range m.targetNodes {
		if node.Path != filepath.Join(root, "repo", "src") {
			continue
		}
		found = true
		if len(node.SessionIDs) != 1 || node.SessionIDs[0] != "sid-1" {
			t.Fatalf("unexpected target node session ids: %#v", node.SessionIDs)
		}
	}
	if !found {
		t.Fatalf("expected target node %q", filepath.Join(root, "repo", "src"))
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
