package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	mv "codecom/internal/move"
	"codecom/internal/search"
	"codecom/internal/sessionindex"
)

const (
	panelSource = iota
	panelTarget
	panelSessions
	defaultWidth  = 120
	defaultHeight = 40
	minPaneHeight = 5
)

type paneState struct {
	cursor int
	offset int
}

type scanFunc func(string) (sessionindex.ScanResult, error)
type buildPlanFunc func(string, string, []sessionindex.SessionRecord) (mv.Plan, error)
type executePlanFunc func(string, mv.Plan) (mv.ExecuteResult, error)
type buildSearchIndexFunc func(string, []sessionindex.SessionRecord) (search.Backend, error)

type searchBuildDoneMsg struct {
	backend search.Backend
	err     error
}

type searchResultMsg struct {
	result search.Result
	err    error
	query  string
}

type Model struct {
	sourceNodes []treeNode
	targetNodes []treeNode
	sessions    []sessionindex.SessionRecord
	selected    map[string]struct{}

	sourceViewRoot  string
	targetBaseRoot  string
	targetViewRoot  string
	targetExpanded  map[string]struct{}
	knownCounts     map[string]int
	knownSessionIDs map[string][]string
	codexRoot       string
	scanSessions    scanFunc
	buildPlan       buildPlanFunc
	executePlan     executePlanFunc

	sourcePane      paneState
	targetPane      paneState
	sessionPane     paneState
	activePanel     int
	filterMode      bool
	sourceFilter    string
	targetFilter    string
	popupOpen       bool
	popupTitle      string
	popupSession    string
	popupOffsets    []int64
	popupHitOffsets map[int64]struct{}
	popupStatic     []string
	popupPane       paneState

	searchBuild      buildSearchIndexFunc
	searchBackend    search.Backend
	searchMode       bool
	searchQuery      string
	searchBuilding   bool
	searchReady      bool
	searchSessionSet map[string]struct{}
	searchFolderSet  map[string]struct{}
	searchOffsets    map[string][]int64

	moveConfirmOpen bool
	status          string
	width           int
	height          int
	styles          styles
}

func NewModel(records []sessionindex.SessionRecord) Model {
	return NewModelWithTargetRoot(records, detectTargetRoot())
}

func NewModelWithTargetRoot(records []sessionindex.SessionRecord, targetRoot string) Model {
	knownCounts := buildKnownSessionCounts(records)
	knownSessionIDs := buildKnownSessionIDs(records)
	expanded := map[string]struct{}{filepath.Clean(targetRoot): {}}
	targetNodes, err := buildTargetNodes(targetRoot, expanded, knownCounts, knownSessionIDs)
	status := "ready"
	if err != nil {
		status = fmt.Sprintf("target root unreadable: %v", err)
		targetNodes = []treeNode{{Path: filepath.Clean(targetRoot), Name: filepath.Base(filepath.Clean(targetRoot)), Depth: 0}}
		if targetNodes[0].Name == "." || targetNodes[0].Name == "" {
			targetNodes[0].Name = filepath.Clean(targetRoot)
		}
	}
	return Model{
		sourceNodes:      buildSourceTree(records),
		targetNodes:      targetNodes,
		sessions:         records,
		selected:         make(map[string]struct{}),
		targetBaseRoot:   filepath.Clean(targetRoot),
		targetViewRoot:   filepath.Clean(targetRoot),
		targetExpanded:   expanded,
		knownCounts:      knownCounts,
		knownSessionIDs:  knownSessionIDs,
		scanSessions:     sessionindex.Scan,
		buildPlan:        mv.BuildPlan,
		executePlan:      mv.ExecutePlan,
		searchBuild:      search.BuildSQLiteIndex,
		activePanel:      panelSource,
		status:           status,
		width:            defaultWidth,
		height:           defaultHeight,
		styles:           newStyles(),
		searchSessionSet: make(map[string]struct{}),
		searchFolderSet:  make(map[string]struct{}),
		searchOffsets:    make(map[string][]int64),
	}
}

func (m Model) WithCodexRoot(root string) Model {
	m.codexRoot = filepath.Clean(root)
	if m.codexRoot != "" && m.codexRoot != "." && m.searchBuild != nil {
		m.searchBuilding = true
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if m.codexRoot == "" || m.codexRoot == "." || m.searchBuild == nil {
		return nil
	}
	return m.buildSearchIndexCmd()
}

func (m Model) buildSearchIndexCmd() tea.Cmd {
	if m.codexRoot == "" || m.codexRoot == "." || m.searchBuild == nil {
		return nil
	}
	records := append([]sessionindex.SessionRecord(nil), m.sessions...)
	root := m.codexRoot
	build := m.searchBuild
	return func() tea.Msg {
		backend, err := build(root, records)
		return searchBuildDoneMsg{backend: backend, err: err}
	}
}

func (m Model) searchQueryCmd(query string) tea.Cmd {
	if m.searchBackend == nil {
		return nil
	}
	backend := m.searchBackend
	q := query
	return func() tea.Msg {
		res, err := backend.Search(q)
		return searchResultMsg{result: res, err: err, query: q}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampAll()
	case searchBuildDoneMsg:
		m.searchBuilding = false
		if msg.err != nil {
			m.searchReady = false
			m.status = fmt.Sprintf("search index unavailable: %v", msg.err)
			return m, nil
		}
		if m.searchBackend != nil {
			_ = m.searchBackend.Close()
		}
		m.searchBackend = msg.backend
		m.searchReady = true
		m.status = "search index ready"
		if m.searchQuery != "" {
			return m, m.searchQueryCmd(m.searchQuery)
		}
		m.clampAll()
		return m, nil
	case searchResultMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("search failed: %v", msg.err)
			return m, nil
		}
		if msg.query != m.searchQuery {
			return m, nil
		}
		m.searchSessionSet = msg.result.SessionIDs
		m.searchFolderSet = msg.result.FolderPaths
		m.searchOffsets = msg.result.OffsetsBySessionID
		m.clampAll()
		m.status = fmt.Sprintf("search results: %d session(s)", len(m.searchSessionSet))
		return m, nil
	case tea.KeyMsg:
		if m.moveConfirmOpen {
			return m.updateMoveConfirm(msg)
		}
		if m.popupOpen {
			return m.updatePopup(msg)
		}
		if m.searchMode {
			return m.updateSearch(msg)
		}
		if m.filterMode {
			return m.updateFilter(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "ctrl+f":
			m.enterSearchMode()
		case "/":
			m.enterFilterMode()
		case "tab":
			m.activePanel = nextPanel(m.activePanel)
		case "shift+tab":
			m.activePanel = prevPanel(m.activePanel)
		case "left":
			m.handleLeft()
		case "right":
			m.handleRight()
		case "up":
			m.moveCursor(-1)
		case "down":
			m.moveCursor(1)
		case "pgup":
			m.moveCursor(-m.pageSize())
		case "pgdown":
			m.moveCursor(m.pageSize())
		case "home":
			m.jumpCursor(0)
		case "end":
			m.jumpCursor(m.activeLen() - 1)
		case "enter":
			m.enterCurrentNode()
		case "f5":
			m.refreshFromDisk()
			if cmd := m.buildSearchIndexCmd(); cmd != nil {
				m.searchBuilding = true
				return m, cmd
			}
		case "f6":
			m.openMoveConfirmation()
		case "space", " ":
			m.toggleCurrentSessionSelection()
		case "a":
			m.selectAllCurrentSourceSessions()
		case "u":
			m.status = "undo requested"
		case "y":
			m.status = "copy report requested"
		}
	}
	return m, nil
}

func (m *Model) handleLeft() {
	switch m.activePanel {
	case panelSessions:
		m.activePanel = panelSource
	case panelTarget:
		if m.collapseOrAscendTarget() {
			return
		}
		m.activePanel = panelSource
	default:
		m.activePanel = panelSource
	}
}

func (m *Model) handleRight() {
	switch m.activePanel {
	case panelSessions:
		m.activePanel = panelTarget
	case panelSource:
		m.activePanel = panelTarget
	case panelTarget:
		m.expandOrDescendTarget()
	}
}

func (m Model) CurrentSourceFolder() string {
	nodes := m.visibleSourceNodes()
	if len(nodes) == 0 {
		return ""
	}
	idx := m.sourcePane.cursor
	if idx < 0 || idx >= len(nodes) {
		return ""
	}
	return nodes[idx].Path
}

func (m Model) CurrentTargetFolder() string {
	nodes := m.visibleTargetNodes()
	if len(nodes) == 0 {
		return ""
	}
	idx := m.targetPane.cursor
	if idx < 0 || idx >= len(nodes) {
		return ""
	}
	return nodes[idx].Path
}

func (m Model) SessionsForCurrentSource() []sessionindex.SessionRecord {
	source := m.CurrentSourceFolder()
	if source == "" {
		return nil
	}
	out := make([]sessionindex.SessionRecord, 0)
	sourceClean := filepath.Clean(source)
	sourcePrefix := sourceClean + string(filepath.Separator)
	for _, s := range m.sessions {
		cwd := filepath.Clean(s.EffectiveCWD())
		if cwd == sourceClean || strings.HasPrefix(cwd+string(filepath.Separator), sourcePrefix) {
			if m.searchActive() {
				if _, ok := m.searchSessionSet[s.SessionID]; !ok {
					continue
				}
			}
			out = append(out, s)
		}
	}
	return out
}

func (m Model) searchActive() bool {
	return strings.TrimSpace(m.searchQuery) != ""
}

func (m Model) View() string {
	main := m.renderMainView()
	if m.moveConfirmOpen {
		return m.renderMoveConfirmation(main)
	}
	if !m.popupOpen {
		return main
	}
	return m.renderConversationPopup()
}

func (m Model) renderMainView() string {
	w, h := m.dimensions()
	topH, bottomH := splitHeights(h)
	colW := max(10, (w-1)/2)
	rightW := max(10, w-colW-1)

	left := m.renderTreePane("Source", m.visibleSourceNodes(), m.sourcePane, colW, topH, m.activePanel == panelSource, true)
	right := m.renderTreePane("Target", m.visibleTargetNodes(), m.targetPane, rightW, topH, m.activePanel == panelTarget, false)
	top := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	bottom := m.renderSessionsPane(colW+rightW, bottomH, m.activePanel == panelSessions)
	statusText := fmt.Sprintf("Status: %s | Selected: %d", m.status, m.SelectedCount())
	if m.searchBuilding {
		statusText = fmt.Sprintf("%s | Search: indexing", statusText)
	} else if m.searchReady {
		statusText = fmt.Sprintf("%s | Search: ready", statusText)
	}
	if m.searchActive() {
		statusText = fmt.Sprintf("%s | Query: %s", statusText, m.searchQuery)
	}
	if m.filterMode {
		statusText = fmt.Sprintf("%s | Filter: %s", statusText, m.currentFilter())
	}
	if m.searchMode {
		statusText = fmt.Sprintf("%s | Search input", statusText)
	}
	status := m.styles.statusBar.Render(padRight(truncateRight(statusText, colW+rightW), colW+rightW))
	keys := m.renderKeyBar(colW + rightW)

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom, status, keys)
}

func (m *Model) moveCursor(delta int) {
	if m.activeLen() == 0 {
		return
	}
	pane := m.activePane()
	pane.cursor += delta
	if pane.cursor < 0 {
		pane.cursor = 0
	}
	if pane.cursor >= m.activeLen() {
		pane.cursor = m.activeLen() - 1
	}
	m.ensureVisible(pane, m.activeViewportHeight())
	m.syncSourceSelection()
}

func (m *Model) jumpCursor(idx int) {
	if m.activeLen() == 0 {
		return
	}
	pane := m.activePane()
	if idx < 0 {
		idx = 0
	}
	if idx >= m.activeLen() {
		idx = m.activeLen() - 1
	}
	pane.cursor = idx
	m.ensureVisible(pane, m.activeViewportHeight())
	m.syncSourceSelection()
}

func (m *Model) syncSourceSelection() {
	if m.activePanel != panelSource {
		m.ensureVisible(&m.sessionPane, m.sessionViewportHeight())
		return
	}
	m.sessionPane.cursor = 0
	m.sessionPane.offset = 0
	m.clampPane(&m.sessionPane, len(m.SessionsForCurrentSource()), m.sessionViewportHeight())
}

func (m *Model) activePane() *paneState {
	switch m.activePanel {
	case panelTarget:
		return &m.targetPane
	case panelSessions:
		return &m.sessionPane
	default:
		return &m.sourcePane
	}
}

func (m Model) activeLen() int {
	switch m.activePanel {
	case panelTarget:
		return len(m.visibleTargetNodes())
	case panelSessions:
		return len(m.SessionsForCurrentSource())
	default:
		return len(m.visibleSourceNodes())
	}
}

func (m Model) pageSize() int {
	if n := m.activeViewportHeight(); n > 1 {
		return n - 1
	}
	return 1
}

func (m Model) activeViewportHeight() int {
	switch m.activePanel {
	case panelSessions:
		return m.sessionViewportHeight()
	default:
		return m.topViewportHeight()
	}
}

func (m *Model) clampAll() {
	m.clampPane(&m.sourcePane, len(m.visibleSourceNodes()), m.topViewportHeight())
	m.clampPane(&m.targetPane, len(m.visibleTargetNodes()), m.topViewportHeight())
	m.clampPane(&m.sessionPane, len(m.SessionsForCurrentSource()), m.sessionViewportHeight())
}

func (m *Model) clampPane(p *paneState, total int, height int) {
	if total <= 0 {
		p.cursor = 0
		p.offset = 0
		return
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= total {
		p.cursor = total - 1
	}
	m.ensureVisible(p, height)
}

func (m *Model) ensureVisible(p *paneState, height int) {
	if height < 1 {
		height = 1
	}
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+height {
		p.offset = p.cursor - height + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

func (m Model) renderTreePane(title string, nodes []treeNode, pane paneState, width, height int, active bool, orphanable bool) string {
	if active && (title == "Source" || title == "Target") && m.currentFilterForTitle(title) != "" {
		title = fmt.Sprintf("%s /%s", title, m.currentFilterForTitle(title))
	}
	lines := make([]string, 0, len(nodes))
	orphan := make(map[int]bool)
	for i, node := range nodes {
		label := strings.Repeat("  ", node.Depth) + fitPathLabel(node.Name, max(1, width-6))
		if node.KnownSessionCount > 1 {
			label = fmt.Sprintf("%s [%d]", label, node.KnownSessionCount)
		}
		lines = append(lines, label)
		if orphanable && node.Orphan {
			orphan[i] = true
		}
	}
	return m.renderPane(title, width, height, lines, pane, active, orphan, nil)
}

func (m Model) renderSessionsPane(width, height int, active bool) string {
	rows := m.SessionsForCurrentSource()
	lines := make([]string, 0, len(rows))
	orphan := make(map[int]bool, len(rows))
	selected := make(map[int]bool, len(rows))
	for i, r := range rows {
		marker := " "
		if m.isSelected(r) {
			marker = "*"
			selected[i] = true
		}
		label := truncateRight(r.DisplayLabel(), max(12, innerSessionLabelWidth(width)))
		meta := truncateRight(sessionMetaLabel(r), max(12, width-len([]rune(label))-8))
		lines = append(lines, fmt.Sprintf("[%s] %s  %s", marker, label, meta))
		orphan[i] = r.Orphan
	}
	return m.renderPane("Sessions", width, height, lines, m.sessionPane, active, orphan, selected)
}

func (m Model) renderPane(title string, width, height int, lines []string, pane paneState, active bool, orphan map[int]bool, selected map[int]bool) string {
	frame := m.styles.inactivePane
	titleStyle := m.styles.inactiveTitle
	if active {
		frame = m.styles.activePane
		titleStyle = m.styles.activeTitle
	}

	innerWidth := max(1, width-frame.GetHorizontalFrameSize())
	innerHeight := max(1, height-frame.GetVerticalFrameSize())
	titleLine := titleStyle.Width(innerWidth).Render(" " + title + " ")
	bodyH := max(1, innerHeight-lipgloss.Height(titleLine))
	visible := visibleSlice(lines, pane.offset, bodyH)
	rows := make([]string, 0, bodyH)
	for i := 0; i < bodyH; i++ {
		if i >= len(visible) {
			rows = append(rows, "")
			continue
		}
		idx := pane.offset + i
		line := visible[i]
		style := m.styles.row
		if selected != nil && selected[idx] {
			style = m.styles.markedRow
		}
		if orphan != nil && orphan[idx] {
			style = m.styles.orphanRow
		}
		if idx == pane.cursor {
			if active {
				style = m.styles.selectedActive
			} else {
				style = m.styles.selectedInactive
			}
			if orphan != nil && orphan[idx] {
				style = style.Foreground(m.styles.colors.orphan)
			}
		}
		rows = append(rows, style.Width(innerWidth).Render(truncateRight(line, innerWidth)))
	}
	content := strings.Join(rows, "\n")
	frame = frame.Width(innerWidth).Height(innerHeight)
	return frame.Render(lipgloss.JoinVertical(lipgloss.Left, titleLine, content))
}

func (m Model) dimensions() (int, int) {
	w := m.width
	h := m.height
	if w < 40 {
		w = defaultWidth
	}
	if h < 12 {
		h = defaultHeight
	}
	return w, h
}

func (m Model) topViewportHeight() int {
	_, h := m.dimensions()
	top, _ := splitHeights(h)
	return m.paneBodyHeight(top)
}

func (m Model) sessionViewportHeight() int {
	_, h := m.dimensions()
	_, bottom := splitHeights(h)
	return m.paneBodyHeight(bottom)
}

func (m Model) paneBodyHeight(outerHeight int) int {
	frameHeight := m.styles.inactivePane.GetVerticalFrameSize()
	titleHeight := 1
	return max(1, outerHeight-frameHeight-titleHeight)
}

func splitHeights(total int) (int, int) {
	usable := max(minPaneHeight*2, total-2)
	top := usable * 55 / 100
	bottom := usable - top
	if top < minPaneHeight {
		top = minPaneHeight
		bottom = usable - top
	}
	if bottom < minPaneHeight {
		bottom = minPaneHeight
		top = usable - bottom
	}
	return top, bottom
}

func visibleSlice(lines []string, offset, height int) []string {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		return nil
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end]
}

func truncateRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	return string(r[:width-1]) + "…"
}

func fitPathLabel(label string, width int) string {
	if width <= 0 {
		return ""
	}
	if len([]rune(label)) <= width {
		return label
	}
	parts := strings.Split(label, string(filepath.Separator))
	if len(parts) <= 2 {
		return truncateRight(label, width)
	}
	candidate := parts[0] + string(filepath.Separator) + "..." + string(filepath.Separator) + parts[len(parts)-1]
	if len([]rune(candidate)) <= width {
		return candidate
	}
	return truncateRight(candidate, width)
}

func innerSessionLabelWidth(width int) int {
	// Leave room for marker, spacing, and a short metadata suffix.
	return max(12, width/2)
}

func sessionMetaLabel(r sessionindex.SessionRecord) string {
	parts := make([]string, 0, 4)
	if r.Model != "" {
		parts = append(parts, r.Model)
	}
	if !r.LastActivity.IsZero() {
		parts = append(parts, r.LastActivity.Format("2006-01-02 15:04"))
	}
	if r.UserMessageCount > 0 {
		parts = append(parts, fmt.Sprintf("%d msg", r.UserMessageCount))
	}
	if r.Aborted {
		parts = append(parts, "aborted")
	}
	if len(parts) == 0 {
		return r.SessionID
	}
	return strings.Join(parts, " | ")
}

func padRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func padVisibleRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	visible := lipgloss.Width(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

func (m Model) renderKeyBar(width int) string {
	if width <= 0 {
		return ""
	}
	segments := make([]string, 0, len(KeyHints))
	for _, hint := range KeyHints {
		segment := m.styles.keyCap.Render(hint.Key) + m.styles.keyLabel.Render(hint.Label)
		segments = append(segments, segment)
	}
	line := ""
	for _, segment := range segments {
		next := segment
		if line != "" {
			next = line + " " + segment
		}
		if lipgloss.Width(next) > width {
			break
		}
		line = next
	}
	return m.styles.keyBar.Render(padVisibleRight(line, width))
}

func (m Model) SelectedCount() int {
	return len(m.selected)
}

func (m Model) isSelected(r sessionindex.SessionRecord) bool {
	_, ok := m.selected[r.SessionFile]
	return ok
}

func (m *Model) toggleCurrentSessionSelection() {
	if m.activePanel != panelSessions {
		m.status = "selection only works in sessions pane"
		return
	}
	rows := m.SessionsForCurrentSource()
	if len(rows) == 0 || m.sessionPane.cursor < 0 || m.sessionPane.cursor >= len(rows) {
		m.status = "no session to select"
		return
	}
	row := rows[m.sessionPane.cursor]
	if _, ok := m.selected[row.SessionFile]; ok {
		delete(m.selected, row.SessionFile)
		m.status = "session unselected"
		return
	}
	m.selected[row.SessionFile] = struct{}{}
	m.status = "session selected"
}

func (m *Model) selectAllCurrentSourceSessions() {
	rows := m.SessionsForCurrentSource()
	if len(rows) == 0 {
		m.status = "no sessions in current source"
		return
	}
	for _, row := range rows {
		m.selected[row.SessionFile] = struct{}{}
	}
	m.status = fmt.Sprintf("selected %d session(s) in current source", len(rows))
}

func (m *Model) selectedSessions() []sessionindex.SessionRecord {
	if len(m.selected) == 0 {
		return nil
	}
	out := make([]sessionindex.SessionRecord, 0, len(m.selected))
	for _, s := range m.sessions {
		if _, ok := m.selected[s.SessionFile]; ok {
			out = append(out, s)
		}
	}
	return out
}

func (m *Model) openMoveConfirmation() {
	if m.codexRoot == "" || m.codexRoot == "." {
		m.status = "move unavailable: codex root not configured"
		return
	}
	selected := m.selectedSessions()
	if len(selected) == 0 {
		m.status = "move blocked: no selected sessions"
		return
	}
	source := m.CurrentSourceFolder()
	target := m.CurrentTargetFolder()
	if source == "" || target == "" {
		m.status = "move blocked: source/target folder is empty"
		return
	}
	m.moveConfirmOpen = true
	m.status = fmt.Sprintf("confirm move: %d session(s) | enter/y confirm | esc/n cancel", len(selected))
}

func (m *Model) performMove() {
	if m.codexRoot == "" || m.codexRoot == "." {
		m.status = "move unavailable: codex root not configured"
		return
	}
	selected := m.selectedSessions()
	if len(selected) == 0 {
		m.status = "move blocked: no selected sessions"
		return
	}
	source := m.CurrentSourceFolder()
	target := m.CurrentTargetFolder()
	if source == "" || target == "" {
		m.status = "move blocked: source/target folder is empty"
		return
	}
	plan, err := m.buildPlan(source, target, selected)
	if err != nil {
		m.status = fmt.Sprintf("move planning failed: %v", err)
		return
	}
	res, err := m.executePlan(m.codexRoot, plan)
	if err != nil {
		if verr, ok := err.(*mv.ValidationErrors); ok {
			m.status = fmt.Sprintf("move blocked: %d validation error(s)", len(verr.Items))
			return
		}
		m.status = fmt.Sprintf("move failed: %v", err)
		return
	}
	m.selected = make(map[string]struct{})
	m.refreshFromDisk()
	if m.status == "refresh completed" {
		msg := fmt.Sprintf("move complete: %d commit(s)", res.FileCommits)
		if res.SnapshotCommitted {
			msg += ", snapshot created"
		}
		m.status = msg
	}
}

func (m Model) updateMoveConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y":
		m.moveConfirmOpen = false
		m.performMove()
	case "esc", "n":
		m.moveConfirmOpen = false
		m.status = "move canceled"
	}
	return m, nil
}

func (m Model) renderMoveConfirmation(_ string) string {
	w, h := m.dimensions()
	boxW := min(max(40, w-12), 84)
	frame := m.styles.activePane.
		Width(max(1, boxW-m.styles.activePane.GetHorizontalFrameSize())).
		Height(max(1, 7-m.styles.activePane.GetVerticalFrameSize()))
	source := truncateRight(m.CurrentSourceFolder(), max(10, boxW-16))
	target := truncateRight(m.CurrentTargetFolder(), max(10, boxW-16))
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.styles.activeTitle.Render(" Confirm Move "),
		fmt.Sprintf("Source: %s", source),
		fmt.Sprintf("Target: %s", target),
		fmt.Sprintf("Selected sessions: %d", m.SelectedCount()),
		"Enter/Y confirm, Esc/N cancel",
	)
	box := frame.Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) refreshFromDisk() {
	if m.codexRoot == "" || m.codexRoot == "." {
		m.status = "refresh unavailable: codex root not configured"
		return
	}
	res, err := m.scanSessions(m.codexRoot)
	if err != nil {
		m.status = fmt.Sprintf("refresh failed: %v", err)
		return
	}
	m.sessions = res.Sessions
	m.knownCounts = buildKnownSessionCounts(m.sessions)
	m.knownSessionIDs = buildKnownSessionIDs(m.sessions)
	m.sourceNodes = buildSourceTree(m.sessions)
	m.reloadTargetNodes("")
	m.pruneSelectedSet()
	m.clampAll()
	m.status = "refresh completed"
}

func (m *Model) pruneSelectedSet() {
	if len(m.selected) == 0 {
		return
	}
	known := make(map[string]struct{}, len(m.sessions))
	for _, s := range m.sessions {
		known[s.SessionFile] = struct{}{}
	}
	for path := range m.selected {
		if _, ok := known[path]; !ok {
			delete(m.selected, path)
		}
	}
}

func (m *Model) collapseOrAscendTarget() bool {
	nodes := m.visibleTargetNodes()
	if len(nodes) == 0 {
		return false
	}
	node := nodes[m.targetPane.cursor]
	if node.ParentNav {
		return false
	}
	if node.Path == m.targetViewRoot && node.Depth == 0 {
		return false
	}
	if node.Expanded {
		delete(m.targetExpanded, node.Path)
		m.reloadTargetNodes("")
		return true
	}
	for i := m.targetPane.cursor - 1; i >= 0; i-- {
		if nodes[i].Depth == node.Depth-1 {
			m.targetPane.cursor = i
			m.ensureVisible(&m.targetPane, m.topViewportHeight())
			return true
		}
	}
	return false
}

func (m *Model) expandOrDescendTarget() {
	nodes := m.visibleTargetNodes()
	if len(nodes) == 0 {
		return
	}
	node := nodes[m.targetPane.cursor]
	if node.HasChildren && !node.Expanded {
		m.targetExpanded[node.Path] = struct{}{}
		m.reloadTargetNodes("")
		return
	}
	if node.Expanded && m.targetPane.cursor+1 < len(nodes) && nodes[m.targetPane.cursor+1].Depth == node.Depth+1 {
		m.targetPane.cursor++
		m.ensureVisible(&m.targetPane, m.topViewportHeight())
	}
}

func (m *Model) reloadTargetNodes(statusPrefix string) {
	nodes, err := buildTargetNodes(m.targetViewRoot, m.targetExpanded, m.knownCounts, m.knownSessionIDs)
	if err != nil {
		m.status = strings.TrimSpace(statusPrefix + " target read error: " + err.Error())
		return
	}
	m.targetNodes = nodes
	m.clampPane(&m.targetPane, len(m.visibleTargetNodes()), m.topViewportHeight())
	if statusPrefix != "" {
		m.status = strings.TrimSpace(statusPrefix)
	}
}

func (m Model) visibleSourceNodes() []treeNode {
	nodes := scopeSourceNodes(m.sourceNodes, m.sourceViewRoot)
	nodes = pruneSourceNodesByMatches(nodes, m.searchFolderSet, m.searchActive())
	return filterTreeNodes(nodes, m.sourceFilter)
}

func (m Model) visibleTargetNodes() []treeNode {
	return filterTreeNodes(scopeTargetNodes(m.targetNodes, m.targetBaseRoot, m.targetViewRoot), m.targetFilter)
}

func pruneSourceNodesByMatches(nodes []treeNode, folders map[string]struct{}, enabled bool) []treeNode {
	if !enabled || len(folders) == 0 {
		return nodes
	}
	out := make([]treeNode, 0, len(nodes))
	for _, node := range nodes {
		if node.ParentNav {
			out = append(out, node)
			continue
		}
		if sourceNodeContainsMatch(node.Path, folders) {
			out = append(out, node)
		}
	}
	return out
}

func sourceNodeContainsMatch(path string, folders map[string]struct{}) bool {
	clean := filepath.Clean(path)
	for folder := range folders {
		f := filepath.Clean(folder)
		if clean == f {
			return true
		}
		prefix := clean + string(filepath.Separator)
		if strings.HasPrefix(f+string(filepath.Separator), prefix) {
			return true
		}
	}
	return false
}

func filterTreeNodes(nodes []treeNode, query string) []treeNode {
	if query == "" {
		return nodes
	}
	out := make([]treeNode, 0, len(nodes))
	for _, node := range nodes {
		if fuzzyMatch(query, node.Name) || fuzzyMatch(query, filepath.Base(node.Path)) {
			out = append(out, node)
		}
	}
	return out
}

func fuzzyMatch(query, candidate string) bool {
	query = normalizeFilterText(query)
	candidate = normalizeFilterText(candidate)
	if query == "" {
		return true
	}
	q := []rune(query)
	qi := 0
	for _, r := range []rune(candidate) {
		if qi < len(q) && r == q[qi] {
			qi++
		}
	}
	return qi == len(q)
}

func normalizeFilterText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func (m *Model) enterFilterMode() {
	if m.activePanel != panelSource && m.activePanel != panelTarget {
		m.status = "filter only works in source and target panes"
		return
	}
	m.filterMode = true
	if m.currentFilter() == "" {
		m.status = "filter: type to search, enter to keep, esc to clear"
	} else {
		m.status = fmt.Sprintf("filter: %s", m.currentFilter())
	}
}

func (m *Model) enterSearchMode() {
	m.searchMode = true
	if m.searchQuery == "" {
		m.status = "search: type query, enter to apply, esc to clear"
		return
	}
	m.status = fmt.Sprintf("search: %s", m.searchQuery)
}

func (m *Model) enterCurrentNode() {
	switch m.activePanel {
	case panelSource:
		m.enterSourceNode()
	case panelTarget:
		m.enterTargetNode()
	case panelSessions:
		m.openCurrentSessionConversation()
	}
}

func (m *Model) enterSourceNode() {
	nodes := m.visibleSourceNodes()
	if len(nodes) == 0 || m.sourcePane.cursor < 0 || m.sourcePane.cursor >= len(nodes) {
		return
	}
	node := nodes[m.sourcePane.cursor]
	previousRoot := m.sourceViewRoot
	selectPath := ""
	if node.ParentNav {
		m.sourceViewRoot = sourceParentRoot(m.sourceNodes, m.sourceViewRoot)
		selectPath = previousRoot
	} else {
		m.sourceViewRoot = node.Path
	}
	m.sourceFilter = ""
	m.filterMode = false
	if selectPath == "" || !m.selectSourcePath(selectPath) {
		m.resetSourcePaneCursor()
	}
	m.sessionPane.cursor = 0
	m.sessionPane.offset = 0
	m.clampPane(&m.sessionPane, len(m.SessionsForCurrentSource()), m.sessionViewportHeight())
	m.status = fmt.Sprintf("source entered: %s", m.CurrentSourceFolder())
}

func (m *Model) enterTargetNode() {
	nodes := m.visibleTargetNodes()
	if len(nodes) == 0 || m.targetPane.cursor < 0 || m.targetPane.cursor >= len(nodes) {
		return
	}
	node := nodes[m.targetPane.cursor]
	previousRoot := m.targetViewRoot
	selectPath := ""
	if node.ParentNav {
		if filepath.Clean(m.targetViewRoot) == filepath.Clean(m.targetBaseRoot) {
			return
		}
		m.targetViewRoot = filepath.Dir(m.targetViewRoot)
		selectPath = previousRoot
	} else {
		m.targetViewRoot = node.Path
	}
	m.targetFilter = ""
	m.filterMode = false
	// Keep MC-style "enter folder" behavior: entered root should show its children immediately.
	m.targetExpanded[m.targetViewRoot] = struct{}{}
	m.reloadTargetNodes("")
	if selectPath == "" || !m.selectTargetPath(selectPath) {
		m.resetTargetPaneCursor()
	}
	m.status = fmt.Sprintf("target entered: %s", m.CurrentTargetFolder())
}

func (m *Model) openCurrentSessionConversation() {
	rows := m.SessionsForCurrentSource()
	if len(rows) == 0 || m.sessionPane.cursor < 0 || m.sessionPane.cursor >= len(rows) {
		m.status = "no session to open"
		return
	}
	row := rows[m.sessionPane.cursor]
	offsets, err := loadConversationOffsets(row)
	if err != nil {
		m.status = fmt.Sprintf("open session failed: %v", err)
		return
	}
	m.popupTitle = row.DisplayLabel()
	m.popupSession = row.SessionFile
	m.popupOffsets = offsets
	m.popupHitOffsets = nil
	m.popupStatic = nil
	if len(offsets) == 0 {
		m.popupStatic = []string{"No conversation messages found in this session."}
	}
	m.popupPane = paneState{}
	m.popupOpen = true
	if m.searchActive() {
		if hits, ok := m.searchOffsets[row.SessionID]; ok && len(hits) > 0 {
			m.popupHitOffsets = make(map[int64]struct{}, len(hits))
			for _, off := range hits {
				m.popupHitOffsets[off] = struct{}{}
			}
			lines := m.popupRenderedLines(m.popupBodyWidth())
			for i, line := range lines {
				if line.Match {
					m.jumpPopupCursor(i)
					break
				}
			}
		}
	}
	m.status = "conversation opened"
}

func (m Model) currentFilter() string {
	switch m.activePanel {
	case panelTarget:
		return m.targetFilter
	default:
		return m.sourceFilter
	}
}

func (m Model) currentSearchQuery() string {
	return m.searchQuery
}

func (m *Model) setCurrentFilter(query string) {
	switch m.activePanel {
	case panelTarget:
		m.targetFilter = query
		m.clampPane(&m.targetPane, len(m.visibleTargetNodes()), m.topViewportHeight())
	default:
		m.sourceFilter = query
		m.clampPane(&m.sourcePane, len(m.visibleSourceNodes()), m.topViewportHeight())
		m.sessionPane.cursor = 0
		m.sessionPane.offset = 0
		m.clampPane(&m.sessionPane, len(m.SessionsForCurrentSource()), m.sessionViewportHeight())
	}
}

func (m *Model) setSearchQuery(query string) {
	m.searchQuery = query
}

func (m Model) currentFilterForTitle(title string) string {
	switch title {
	case "Source":
		return m.sourceFilter
	case "Target":
		return m.targetFilter
	default:
		return ""
	}
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.setCurrentFilter("")
		m.filterMode = false
		m.status = "filter cleared"
	case "enter":
		m.filterMode = false
		if m.currentFilter() == "" {
			m.status = "filter closed"
			break
		}
		m.enterCurrentNode()
	case "backspace", "ctrl+h":
		query := m.currentFilter()
		if query != "" {
			r := []rune(query)
			m.setCurrentFilter(string(r[:len(r)-1]))
		}
		m.status = fmt.Sprintf("filter: %s", m.currentFilter())
	default:
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			m.setCurrentFilter(m.currentFilter() + string(msg.Runes))
			m.status = fmt.Sprintf("filter: %s", m.currentFilter())
		}
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.setSearchQuery("")
		m.searchMode = false
		m.searchSessionSet = make(map[string]struct{})
		m.searchFolderSet = make(map[string]struct{})
		m.searchOffsets = make(map[string][]int64)
		m.clampAll()
		m.status = "search cleared"
		return m, nil
	case "enter":
		m.searchMode = false
		if strings.TrimSpace(m.currentSearchQuery()) == "" {
			m.status = "search closed"
			return m, nil
		}
		if !m.searchReady || m.searchBackend == nil {
			m.status = "search index not ready"
			return m, nil
		}
		return m, m.searchQueryCmd(m.currentSearchQuery())
	case "backspace", "ctrl+h":
		q := m.currentSearchQuery()
		if q != "" {
			r := []rune(q)
			m.setSearchQuery(string(r[:len(r)-1]))
		}
		m.status = fmt.Sprintf("search: %s", m.currentSearchQuery())
		return m, nil
	default:
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			m.setSearchQuery(m.currentSearchQuery() + string(msg.Runes))
			m.status = fmt.Sprintf("search: %s", m.currentSearchQuery())
		}
		return m, nil
	}
}

func (m Model) updatePopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.popupOpen = false
		m.popupTitle = ""
		m.popupSession = ""
		m.popupOffsets = nil
		m.popupHitOffsets = nil
		m.popupStatic = nil
		m.popupPane = paneState{}
		m.status = "conversation closed"
	case "up":
		m.movePopupCursor(-1)
	case "down":
		m.movePopupCursor(1)
	case "pgup":
		m.movePopupCursor(-m.popupBodyHeight())
	case "pgdown":
		m.movePopupCursor(m.popupBodyHeight())
	case "home":
		m.jumpPopupCursor(0)
	case "end":
		m.jumpPopupCursor(m.popupLineCount() - 1)
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) movePopupCursor(delta int) {
	if m.popupLineCount() == 0 {
		return
	}
	m.popupPane.cursor += delta
	if m.popupPane.cursor < 0 {
		m.popupPane.cursor = 0
	}
	if m.popupPane.cursor >= m.popupLineCount() {
		m.popupPane.cursor = m.popupLineCount() - 1
	}
	m.ensureVisible(&m.popupPane, m.popupBodyHeight())
}

func (m *Model) jumpPopupCursor(idx int) {
	if m.popupLineCount() == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= m.popupLineCount() {
		idx = m.popupLineCount() - 1
	}
	m.popupPane.cursor = idx
	m.ensureVisible(&m.popupPane, m.popupBodyHeight())
}

func (m Model) renderConversationPopup() string {
	w, h := m.dimensions()
	popupW := max(40, w-8)
	popupH := max(12, h-4)

	frame := m.styles.activePane.
		Width(max(1, popupW-m.styles.activePane.GetHorizontalFrameSize())).
		Height(max(1, popupH-m.styles.activePane.GetVerticalFrameSize()))
	title := m.popupTitle
	if title == "" {
		title = "Conversation"
	}
	header := m.styles.activeTitle.Render(" Conversation: " + truncateRight(title, max(10, popupW-20)) + " ")
	hint := m.styles.inactiveTitle.Render(" Esc close ")
	titleLine := lipgloss.JoinHorizontal(lipgloss.Top, header, hint)

	bodyW := max(1, popupW-m.styles.activePane.GetHorizontalFrameSize())
	bodyH := max(1, popupH-m.styles.activePane.GetVerticalFrameSize()-lipgloss.Height(titleLine))
	lines := m.popupRenderedLines(bodyW)
	total := len(lines)
	rows := make([]string, 0, bodyH)
	for i := 0; i < bodyH; i++ {
		idx := m.popupPane.offset + i
		if idx >= total {
			rows = append(rows, "")
			continue
		}
		style := m.styles.row
		switch lines[idx].Speaker {
		case "You":
			style = m.styles.popupUserRow
		case "Assistant":
			style = m.styles.popupAssistantRow
		}
		if idx == m.popupPane.cursor {
			style = m.styles.selectedActive
		} else if lines[idx].Match {
			style = m.styles.searchHitRow
		}
		rows = append(rows, style.Width(bodyW).Render(lines[idx].Text))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, strings.Join(rows, "\n"))
	popup := frame.Render(content)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, popup)
}

func (m Model) popupBodyHeight() int {
	_, h := m.dimensions()
	popupH := max(12, h-4)
	return max(1, popupH-m.styles.activePane.GetVerticalFrameSize()-1)
}

type popupEventLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

type popupEventPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type popupResponsePayload struct {
	Type    string                `json:"type"`
	Role    string                `json:"role"`
	Content []popupResponseRecord `json:"content"`
}

type popupResponseRecord struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type popupParsedLine struct {
	Text    string
	Speaker string
	Match   bool
}

type popupRenderedLine struct {
	Text    string
	Speaker string
	Match   bool
}

func loadConversationOffsets(r sessionindex.SessionRecord) ([]int64, error) {
	if r.SessionFile == "" {
		return nil, fmt.Errorf("missing session file")
	}
	f, err := os.Open(r.SessionFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	offsets := make([]int64, 0, 64)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 128*1024), 16*1024*1024)
	var offset int64
	lastLine := ""
	for scanner.Scan() {
		raw := append([]byte(nil), scanner.Bytes()...)
		if line, ok := parseConversationLine(raw); ok {
			if line.Text == lastLine {
				offset += int64(len(scanner.Bytes())) + 1
				continue
			}
			offsets = append(offsets, offset)
			lastLine = line.Text
		}
		offset += int64(len(scanner.Bytes())) + 1
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return offsets, nil
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func formatConversationLine(tsRaw, speaker, message string) string {
	if t, err := time.Parse(time.RFC3339, tsRaw); err == nil {
		return fmt.Sprintf("%s  %s: %s", t.Format("2006-01-02 15:04"), speaker, message)
	}
	return fmt.Sprintf("%s: %s", speaker, message)
}

func (m Model) popupLineCount() int {
	return len(m.popupRenderedLines(m.popupBodyWidth()))
}

func (m Model) popupBodyWidth() int {
	w, _ := m.dimensions()
	popupW := max(40, w-8)
	return max(1, popupW-m.styles.activePane.GetHorizontalFrameSize())
}

func (m Model) popupRenderedLines(width int) []popupRenderedLine {
	raw := make([]popupParsedLine, 0, 128)
	if len(m.popupStatic) > 0 {
		for _, line := range m.popupStatic {
			raw = append(raw, popupParsedLine{Text: line})
		}
	} else {
		for _, off := range m.popupOffsets {
			line, err := readConversationLineAt(m.popupSession, off)
			if err != nil || line.Text == "" {
				raw = append(raw, popupParsedLine{Text: "<unavailable>"})
				continue
			}
			if _, ok := m.popupHitOffsets[off]; ok {
				line.Match = true
			}
			raw = append(raw, line)
		}
	}
	out := make([]popupRenderedLine, 0, len(raw))
	for _, line := range raw {
		for _, chunk := range wrapLine(line.Text, width) {
			out = append(out, popupRenderedLine{Text: chunk, Speaker: line.Speaker, Match: line.Match})
		}
	}
	if len(out) == 0 {
		return []popupRenderedLine{{Text: ""}}
	}
	return out
}

func readConversationLineAt(path string, offset int64) (popupParsedLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return popupParsedLine{}, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return popupParsedLine{}, err
	}
	reader := bufio.NewReader(f)
	raw, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return popupParsedLine{}, err
	}
	line, ok := parseConversationLine(raw)
	if !ok {
		return popupParsedLine{}, nil
	}
	return line, nil
}

func parseConversationLine(raw []byte) (popupParsedLine, bool) {
	var ev popupEventLine
	if err := json.Unmarshal(raw, &ev); err != nil {
		return popupParsedLine{}, false
	}
	lineType := ev.Type
	if lineType == "" {
		lineType = ev.EventType
	}
	switch lineType {
	case "response_item":
		var payload popupResponsePayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return popupParsedLine{}, false
		}
		if payload.Type != "message" {
			return popupParsedLine{}, false
		}
		prefix := ""
		switch payload.Role {
		case "user":
			prefix = "You"
		case "assistant":
			prefix = "Assistant"
		default:
			return popupParsedLine{}, false
		}
		parts := make([]string, 0, len(payload.Content))
		for _, c := range payload.Content {
			switch c.Type {
			case "input_text", "output_text", "text":
				text := collapseWhitespace(c.Text)
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) == 0 {
			return popupParsedLine{}, false
		}
		return popupParsedLine{Text: formatConversationLine(ev.Timestamp, prefix, strings.Join(parts, " ")), Speaker: prefix}, true
	case "event_msg":
		var payload popupEventPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return popupParsedLine{}, false
		}
		prefix := ""
		switch payload.Type {
		case "user_message":
			prefix = "You"
		case "assistant_message", "agent_message":
			prefix = "Assistant"
		default:
			return popupParsedLine{}, false
		}
		msg := collapseWhitespace(payload.Message)
		if msg == "" {
			return popupParsedLine{}, false
		}
		return popupParsedLine{Text: formatConversationLine(ev.Timestamp, prefix, msg), Speaker: prefix}, true
	default:
		return popupParsedLine{}, false
	}
}

func wrapLine(s string, width int) []string {
	if width <= 1 {
		return []string{truncateRight(s, max(1, width))}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, 2)
	cur := ""
	for _, w := range words {
		word := w
		for len([]rune(word)) > width {
			chunk := string([]rune(word)[:width])
			word = string([]rune(word)[width:])
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			lines = append(lines, chunk)
		}
		if cur == "" {
			cur = word
			continue
		}
		if len([]rune(cur))+1+len([]rune(word)) <= width {
			cur += " " + word
		} else {
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func scopeSourceNodes(nodes []treeNode, viewRoot string) []treeNode {
	if viewRoot == "" {
		return nodes
	}
	rootIdx := findNodeIndexByPath(nodes, viewRoot)
	if rootIdx < 0 {
		return nodes
	}
	root := nodes[rootIdx]
	out := make([]treeNode, 0, len(nodes)-rootIdx+1)
	if parent := sourceParentRoot(nodes, viewRoot); parent != "" {
		out = append(out, treeNode{Path: parent, Name: "..", ParentNav: true})
	}
	rootNode := root
	rootNode.Depth = 0
	out = append(out, rootNode)
	for i := rootIdx + 1; i < len(nodes); i++ {
		if nodes[i].Depth <= root.Depth {
			break
		}
		child := nodes[i]
		child.Depth -= root.Depth
		out = append(out, child)
	}
	return out
}

func sourceParentRoot(nodes []treeNode, viewRoot string) string {
	if viewRoot == "" {
		return ""
	}
	best := ""
	for _, node := range nodes {
		if node.Path == viewRoot || node.ParentNav {
			continue
		}
		prefix := node.Path + string(filepath.Separator)
		if strings.HasPrefix(viewRoot+string(filepath.Separator), prefix) && len(node.Path) > len(best) {
			best = node.Path
		}
	}
	return best
}

func scopeTargetNodes(nodes []treeNode, baseRoot, viewRoot string) []treeNode {
	if viewRoot == "" || filepath.Clean(viewRoot) == filepath.Clean(baseRoot) {
		return nodes
	}
	out := make([]treeNode, 0, len(nodes)+1)
	out = append(out, treeNode{Path: filepath.Dir(viewRoot), Name: "..", ParentNav: true})
	out = append(out, nodes...)
	return out
}

func findNodeIndexByPath(nodes []treeNode, path string) int {
	for i, node := range nodes {
		if node.Path == path {
			return i
		}
	}
	return -1
}

func (m *Model) resetSourcePaneCursor() {
	m.sourcePane.offset = 0
	if m.sourceViewRoot == "" {
		m.sourcePane.cursor = 0
	} else {
		m.sourcePane.cursor = 1
	}
	m.clampPane(&m.sourcePane, len(m.visibleSourceNodes()), m.topViewportHeight())
}

func (m *Model) resetTargetPaneCursor() {
	m.targetPane.offset = 0
	if filepath.Clean(m.targetViewRoot) == filepath.Clean(m.targetBaseRoot) {
		m.targetPane.cursor = 0
	} else {
		m.targetPane.cursor = 1
	}
	m.clampPane(&m.targetPane, len(m.visibleTargetNodes()), m.topViewportHeight())
}

func (m *Model) selectSourcePath(path string) bool {
	nodes := m.visibleSourceNodes()
	for i, node := range nodes {
		if node.Path == path && !node.ParentNav {
			m.sourcePane.cursor = i
			m.ensureVisible(&m.sourcePane, m.topViewportHeight())
			return true
		}
	}
	return false
}

func (m *Model) selectTargetPath(path string) bool {
	nodes := m.visibleTargetNodes()
	for i, node := range nodes {
		if node.Path == path && !node.ParentNav {
			m.targetPane.cursor = i
			m.ensureVisible(&m.targetPane, m.topViewportHeight())
			return true
		}
	}
	return false
}

func nextPanel(current int) int {
	switch current {
	case panelSource:
		return panelSessions
	case panelSessions:
		return panelTarget
	default:
		return panelSource
	}
}

func prevPanel(current int) int {
	switch current {
	case panelSource:
		return panelTarget
	case panelTarget:
		return panelSessions
	default:
		return panelSource
	}
}

type palette struct {
	chromeBG lipgloss.Color
	paneBG   lipgloss.Color
	text     lipgloss.Color
	accent   lipgloss.Color
	selectBG lipgloss.Color
	selectFG lipgloss.Color
	orphan   lipgloss.Color
	muted    lipgloss.Color
}

type styles struct {
	colors            palette
	activePane        lipgloss.Style
	inactivePane      lipgloss.Style
	activeTitle       lipgloss.Style
	inactiveTitle     lipgloss.Style
	row               lipgloss.Style
	popupUserRow      lipgloss.Style
	popupAssistantRow lipgloss.Style
	searchHitRow      lipgloss.Style
	markedRow         lipgloss.Style
	orphanRow         lipgloss.Style
	selectedActive    lipgloss.Style
	selectedInactive  lipgloss.Style
	statusBar         lipgloss.Style
	keyBar            lipgloss.Style
	keyCap            lipgloss.Style
	keyLabel          lipgloss.Style
}

func newStyles() styles {
	p := palette{
		chromeBG: lipgloss.Color("4"),
		paneBG:   lipgloss.Color("17"),
		text:     lipgloss.Color("15"),
		accent:   lipgloss.Color("14"),
		selectBG: lipgloss.Color("6"),
		selectFG: lipgloss.Color("0"),
		orphan:   lipgloss.Color("9"),
		muted:    lipgloss.Color("153"),
	}
	basePane := lipgloss.NewStyle().
		Background(p.paneBG).
		Foreground(p.text).
		Border(lipgloss.NormalBorder()).
		Padding(0, 0)
	activePane := basePane.Copy().BorderForeground(p.accent)
	inactivePane := basePane.Copy().BorderForeground(p.chromeBG)
	return styles{
		colors:            p,
		activePane:        activePane,
		inactivePane:      inactivePane,
		activeTitle:       lipgloss.NewStyle().Background(p.accent).Foreground(p.selectFG).Bold(true),
		inactiveTitle:     lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text).Bold(true),
		row:               lipgloss.NewStyle().Background(p.paneBG).Foreground(p.text),
		popupUserRow:      lipgloss.NewStyle().Background(lipgloss.Color("18")).Foreground(p.text),
		popupAssistantRow: lipgloss.NewStyle().Background(lipgloss.Color("19")).Foreground(p.text),
		searchHitRow:      lipgloss.NewStyle().Background(lipgloss.Color("22")).Foreground(p.text).Bold(true),
		markedRow:         lipgloss.NewStyle().Background(p.paneBG).Foreground(p.accent).Bold(true),
		orphanRow:         lipgloss.NewStyle().Background(p.paneBG).Foreground(p.orphan),
		selectedActive:    lipgloss.NewStyle().Background(p.selectBG).Foreground(p.selectFG).Bold(true),
		selectedInactive:  lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text).Bold(true),
		statusBar:         lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text),
		keyBar:            lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text),
		keyCap:            lipgloss.NewStyle().Background(p.text).Foreground(p.selectFG).Bold(true).Padding(0, 1),
		keyLabel:          lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text).Padding(0, 1),
	}
}
