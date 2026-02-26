package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"codecom/internal/sessionindex"
)

const (
	panelSource = iota
	panelTarget
)

type Model struct {
	sourceFolders []string
	targetFolders []string
	sessions      []sessionindex.SessionRecord
	folderCounts  map[string]int

	sourceIndex int
	targetIndex int
	activePanel int
	status      string
}

func NewModel(records []sessionindex.SessionRecord) Model {
	sourceSet := make(map[string]struct{})
	folderCounts := make(map[string]int)
	for _, s := range records {
		cwd := s.EffectiveCWD()
		if cwd == "" {
			continue
		}
		sourceSet[cwd] = struct{}{}
		folderCounts[cwd]++
	}

	sources := make([]string, 0, len(sourceSet))
	for p := range sourceSet {
		sources = append(sources, p)
	}
	sort.Strings(sources)

	targets := make([]string, len(sources))
	copy(targets, sources)

	return Model{
		sourceFolders: sources,
		targetFolders: targets,
		sessions:      records,
		folderCounts:  folderCounts,
		activePanel:   panelSource,
		status:        "ready",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			if m.activePanel == panelSource {
				m.activePanel = panelTarget
			} else {
				m.activePanel = panelSource
			}
		case "up":
			if m.activePanel == panelSource && m.sourceIndex > 0 {
				m.sourceIndex--
			}
			if m.activePanel == panelTarget && m.targetIndex > 0 {
				m.targetIndex--
			}
		case "down":
			if m.activePanel == panelSource && m.sourceIndex < len(m.sourceFolders)-1 {
				m.sourceIndex++
			}
			if m.activePanel == panelTarget && m.targetIndex < len(m.targetFolders)-1 {
				m.targetIndex++
			}
		case "f5":
			m.status = "refresh requested"
		case "f6":
			m.status = "move requested (not implemented)"
		case "space":
			m.status = "selection toggle requested"
		case "a":
			m.status = "select-all requested"
		case "u":
			m.status = "undo requested"
		case "y":
			m.status = "copy report requested"
		}
	}
	return m, nil
}

func (m Model) CurrentSourceFolder() string {
	if len(m.sourceFolders) == 0 {
		return ""
	}
	if m.sourceIndex < 0 || m.sourceIndex >= len(m.sourceFolders) {
		return ""
	}
	return m.sourceFolders[m.sourceIndex]
}

func (m Model) SessionsForCurrentSource() []sessionindex.SessionRecord {
	source := m.CurrentSourceFolder()
	if source == "" {
		return nil
	}
	out := make([]sessionindex.SessionRecord, 0)
	sourcePrefix := filepath.Clean(source) + string(filepath.Separator)
	for _, s := range m.sessions {
		cwd := s.EffectiveCWD()
		clean := filepath.Clean(cwd)
		if clean == filepath.Clean(source) || strings.HasPrefix(clean+string(filepath.Separator), sourcePrefix) {
			out = append(out, s)
		}
	}
	return out
}

func (m Model) View() string {
	leftLines := panelLines("Source", m.sourceFolders, m.sourceIndex, m.activePanel == panelSource, m.folderCounts)
	rightLines := panelLines("Target", m.targetFolders, m.targetIndex, m.activePanel == panelTarget, m.folderCounts)
	top := joinColumns(leftLines, rightLines, 52)

	rows := m.SessionsForCurrentSource()
	sessionLines := make([]string, 0, len(rows)+2)
	sessionLines = append(sessionLines, "Sessions (current source)")
	for _, r := range rows {
		marker := " "
		id := r.SessionID
		cwd := r.EffectiveCWD()
		if r.Orphan {
			marker = "!"
			id = red(id)
			cwd = red(cwd)
		}
		sessionLines = append(sessionLines, fmt.Sprintf("[%s] %s  %s", marker, id, cwd))
	}
	if len(rows) == 0 {
		sessionLines = append(sessionLines, "(no sessions for selected source)")
	}

	return top + "\n\n" + strings.Join(sessionLines, "\n") + "\n\n" +
		"Status: " + m.status + "\n" +
		strings.Join(KeyHints, " | ") + "\n"
}

func panelLines(title string, paths []string, selected int, active bool, counts map[string]int) []string {
	header := title
	if active {
		header += " *"
	}
	lines := []string{header}
	if len(paths) == 0 {
		return append(lines, "(empty)")
	}
	for i, p := range paths {
		cursor := " "
		if i == selected {
			cursor = ">"
		}
		label := p
		if n := counts[p]; n > 1 {
			label = fmt.Sprintf("%s [%d]", p, n)
		}
		lines = append(lines, fmt.Sprintf("%s %s", cursor, label))
	}
	return lines
}

func joinColumns(left, right []string, leftWidth int) string {
	max := len(left)
	if len(right) > max {
		max = len(right)
	}
	rows := make([]string, 0, max)
	for i := 0; i < max; i++ {
		l := ""
		r := ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		rows = append(rows, fmt.Sprintf("%-*s | %s", leftWidth, l, r))
	}
	return strings.Join(rows, "\n")
}

func red(s string) string {
	return "\x1b[31m" + s + "\x1b[0m"
}
