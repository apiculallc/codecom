package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

type Model struct {
	sourceNodes []treeNode
	targetNodes []treeNode
	sessions    []sessionindex.SessionRecord
	selected    map[string]struct{}

	targetRoot     string
	targetExpanded map[string]struct{}
	knownCounts    map[string]int

	sourcePane  paneState
	targetPane  paneState
	sessionPane paneState
	activePanel int
	status      string
	width       int
	height      int
	styles      styles
}

func NewModel(records []sessionindex.SessionRecord) Model {
	return NewModelWithTargetRoot(records, detectTargetRoot())
}

func NewModelWithTargetRoot(records []sessionindex.SessionRecord, targetRoot string) Model {
	knownCounts := buildKnownSessionCounts(records)
	expanded := map[string]struct{}{filepath.Clean(targetRoot): {}}
	targetNodes, err := buildTargetNodes(targetRoot, expanded, knownCounts)
	status := "ready"
	if err != nil {
		status = fmt.Sprintf("target root unreadable: %v", err)
		targetNodes = []treeNode{{Path: filepath.Clean(targetRoot), Name: filepath.Base(filepath.Clean(targetRoot)), Depth: 0}}
		if targetNodes[0].Name == "." || targetNodes[0].Name == "" {
			targetNodes[0].Name = filepath.Clean(targetRoot)
		}
	}
	return Model{
		sourceNodes:    buildSourceTree(records),
		targetNodes:    targetNodes,
		sessions:       records,
		selected:       make(map[string]struct{}),
		targetRoot:     filepath.Clean(targetRoot),
		targetExpanded: expanded,
		knownCounts:    knownCounts,
		activePanel:    panelSource,
		status:         status,
		width:          defaultWidth,
		height:         defaultHeight,
		styles:         newStyles(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampAll()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
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
		case "f5":
			m.status = "refresh requested"
		case "f6":
			m.status = "move requested (not implemented)"
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
	if len(m.sourceNodes) == 0 {
		return ""
	}
	idx := m.sourcePane.cursor
	if idx < 0 || idx >= len(m.sourceNodes) {
		return ""
	}
	return m.sourceNodes[idx].Path
}

func (m Model) CurrentTargetFolder() string {
	if len(m.targetNodes) == 0 {
		return ""
	}
	idx := m.targetPane.cursor
	if idx < 0 || idx >= len(m.targetNodes) {
		return ""
	}
	return m.targetNodes[idx].Path
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
			out = append(out, s)
		}
	}
	return out
}

func (m Model) View() string {
	w, h := m.dimensions()
	topH, bottomH := splitHeights(h)
	colW := max(10, (w-1)/2)
	rightW := max(10, w-colW-1)

	left := m.renderTreePane("Source", m.sourceNodes, m.sourcePane, colW, topH, m.activePanel == panelSource, true)
	right := m.renderTreePane("Target", m.targetNodes, m.targetPane, rightW, topH, m.activePanel == panelTarget, false)
	top := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	bottom := m.renderSessionsPane(colW+rightW, bottomH, m.activePanel == panelSessions)
	statusText := fmt.Sprintf("Status: %s | Selected: %d", m.status, m.SelectedCount())
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
		return len(m.targetNodes)
	case panelSessions:
		return len(m.SessionsForCurrentSource())
	default:
		return len(m.sourceNodes)
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
	m.clampPane(&m.sourcePane, len(m.sourceNodes), m.topViewportHeight())
	m.clampPane(&m.targetPane, len(m.targetNodes), m.topViewportHeight())
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

func (m *Model) collapseOrAscendTarget() bool {
	if len(m.targetNodes) == 0 {
		return false
	}
	node := m.targetNodes[m.targetPane.cursor]
	if node.Path == m.targetRoot && node.Depth == 0 {
		return false
	}
	if node.Expanded {
		delete(m.targetExpanded, node.Path)
		m.reloadTargetNodes("")
		return true
	}
	for i := m.targetPane.cursor - 1; i >= 0; i-- {
		if m.targetNodes[i].Depth == node.Depth-1 {
			m.targetPane.cursor = i
			m.ensureVisible(&m.targetPane, m.topViewportHeight())
			return true
		}
	}
	return false
}

func (m *Model) expandOrDescendTarget() {
	if len(m.targetNodes) == 0 {
		return
	}
	node := m.targetNodes[m.targetPane.cursor]
	if node.HasChildren && !node.Expanded {
		m.targetExpanded[node.Path] = struct{}{}
		m.reloadTargetNodes("")
		return
	}
	if node.Expanded && m.targetPane.cursor+1 < len(m.targetNodes) && m.targetNodes[m.targetPane.cursor+1].Depth == node.Depth+1 {
		m.targetPane.cursor++
		m.ensureVisible(&m.targetPane, m.topViewportHeight())
	}
}

func (m *Model) reloadTargetNodes(statusPrefix string) {
	nodes, err := buildTargetNodes(m.targetRoot, m.targetExpanded, m.knownCounts)
	if err != nil {
		m.status = strings.TrimSpace(statusPrefix + " target read error: " + err.Error())
		return
	}
	m.targetNodes = nodes
	m.clampPane(&m.targetPane, len(m.targetNodes), m.topViewportHeight())
	if statusPrefix != "" {
		m.status = strings.TrimSpace(statusPrefix)
	}
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
	colors           palette
	activePane       lipgloss.Style
	inactivePane     lipgloss.Style
	activeTitle      lipgloss.Style
	inactiveTitle    lipgloss.Style
	row              lipgloss.Style
	markedRow        lipgloss.Style
	orphanRow        lipgloss.Style
	selectedActive   lipgloss.Style
	selectedInactive lipgloss.Style
	statusBar        lipgloss.Style
	keyBar           lipgloss.Style
	keyCap           lipgloss.Style
	keyLabel         lipgloss.Style
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
		colors:           p,
		activePane:       activePane,
		inactivePane:     inactivePane,
		activeTitle:      lipgloss.NewStyle().Background(p.accent).Foreground(p.selectFG).Bold(true),
		inactiveTitle:    lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text).Bold(true),
		row:              lipgloss.NewStyle().Background(p.paneBG).Foreground(p.text),
		markedRow:        lipgloss.NewStyle().Background(p.paneBG).Foreground(p.accent).Bold(true),
		orphanRow:        lipgloss.NewStyle().Background(p.paneBG).Foreground(p.orphan),
		selectedActive:   lipgloss.NewStyle().Background(p.selectBG).Foreground(p.selectFG).Bold(true),
		selectedInactive: lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text).Bold(true),
		statusBar:        lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text),
		keyBar:           lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text),
		keyCap:           lipgloss.NewStyle().Background(p.text).Foreground(p.selectFG).Bold(true).Padding(0, 1),
		keyLabel:         lipgloss.NewStyle().Background(p.chromeBG).Foreground(p.text).Padding(0, 1),
	}
}
