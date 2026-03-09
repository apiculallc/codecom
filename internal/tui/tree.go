package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codecom/internal/sessionindex"
)

type treeNode struct {
	Path              string
	Name              string
	Depth             int
	Expanded          bool
	HasChildren       bool
	KnownSessionCount int
	Orphan            bool
	ParentNav         bool
}

type sourceTreeNode struct {
	Path      string
	Name      string
	Children  map[string]*sourceTreeNode
	Order     []string
	Count     int
	Orphan    bool
	HasRecord bool
	Expanded  bool
}

func detectTargetRoot() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return home
	}
	return string(filepath.Separator)
}

func buildKnownSessionCounts(records []sessionindex.SessionRecord) map[string]int {
	counts := make(map[string]int)
	for _, r := range records {
		cwd := filepath.Clean(r.EffectiveCWD())
		if cwd == "." || cwd == "" {
			continue
		}
		counts[cwd]++
	}
	return counts
}

func buildSourceTree(records []sessionindex.SessionRecord) []treeNode {
	root := &sourceTreeNode{Children: make(map[string]*sourceTreeNode), Expanded: true}
	for _, r := range records {
		cwd := filepath.Clean(r.EffectiveCWD())
		if cwd == "." || cwd == "" {
			continue
		}
		parts := splitPathParts(cwd)
		cur := root
		cur.Count++
		accum := volumePrefix(cwd)
		for _, part := range parts {
			if accum == "" || accum == string(filepath.Separator) {
				accum = filepath.Join(accum, part)
			} else {
				accum = filepath.Join(accum, part)
			}
			if cur.Children[part] == nil {
				cur.Children[part] = &sourceTreeNode{
					Path:     accum,
					Name:     part,
					Children: make(map[string]*sourceTreeNode),
					Expanded: true,
				}
				cur.Order = append(cur.Order, part)
			}
			cur = cur.Children[part]
			cur.Count++
		}
		cur.HasRecord = true
		cur.Orphan = r.Orphan
	}
	var out []treeNode
	appendSourceChildren(&out, root, 0)
	return out
}

func appendSourceChildren(out *[]treeNode, parent *sourceTreeNode, depth int) {
	sort.Strings(parent.Order)
	for _, name := range parent.Order {
		child := compactSourceNode(parent.Children[name])
		node := treeNode{
			Path:              child.Path,
			Name:              child.Name,
			Depth:             depth,
			Expanded:          child.Expanded,
			HasChildren:       len(child.Children) > 0,
			KnownSessionCount: child.Count,
			Orphan:            child.Orphan,
		}
		*out = append(*out, node)
		if child.Expanded {
			appendSourceChildren(out, child, depth+1)
		}
	}
}

func compactSourceNode(node *sourceTreeNode) *sourceTreeNode {
	if node == nil {
		return nil
	}
	first := node
	last := node
	parts := []string{first.Name}
	for !last.HasRecord && len(last.Children) == 1 {
		nextName := last.Order[0]
		next := last.Children[nextName]
		if next == nil {
			break
		}
		last = next
		parts = append(parts, last.Name)
	}
	if first == last {
		return node
	}
	copyNode := *last
	copyNode.Name = strings.Join(parts, string(filepath.Separator))
	return &copyNode
}

func buildTargetNodes(root string, expanded map[string]struct{}, knownCounts map[string]int) ([]treeNode, error) {
	root = filepath.Clean(root)
	nodes := []treeNode{{
		Path:              root,
		Name:              filepath.Base(root),
		Depth:             0,
		Expanded:          isExpanded(root, expanded),
		HasChildren:       hasVisibleDirChildren(root),
		KnownSessionCount: knownCounts[root],
	}}
	if nodes[0].Name == "." || nodes[0].Name == string(filepath.Separator) || nodes[0].Name == "" {
		nodes[0].Name = root
	}
	if !nodes[0].Expanded {
		return nodes, nil
	}
	children, err := appendTargetChildren(root, 1, expanded, knownCounts)
	if err != nil {
		return nodes, err
	}
	return append(nodes, children...), nil
}

func appendTargetChildren(path string, depth int, expanded map[string]struct{}, knownCounts map[string]int) ([]treeNode, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	dirs := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !entry.IsDir() {
			continue
		}
		dirs = append(dirs, entry)
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	out := make([]treeNode, 0)
	for _, entry := range dirs {
		childPath := filepath.Join(path, entry.Name())
		node := treeNode{
			Path:              childPath,
			Name:              entry.Name(),
			Depth:             depth,
			Expanded:          isExpanded(childPath, expanded),
			HasChildren:       hasVisibleDirChildren(childPath),
			KnownSessionCount: knownCounts[childPath],
		}
		out = append(out, node)
		if node.Expanded {
			children, err := appendTargetChildren(childPath, depth+1, expanded, knownCounts)
			if err != nil {
				continue
			}
			out = append(out, children...)
		}
	}
	return out, nil
}

func hasVisibleDirChildren(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.IsDir() {
			return true
		}
	}
	return false
}

func isExpanded(path string, expanded map[string]struct{}) bool {
	_, ok := expanded[path]
	return ok
}

func splitPathParts(path string) []string {
	clean := filepath.Clean(path)
	vol := volumePrefix(clean)
	rest := strings.TrimPrefix(clean, vol)
	rest = strings.TrimPrefix(rest, string(filepath.Separator))
	if rest == "" {
		return nil
	}
	return strings.Split(rest, string(filepath.Separator))
}

func volumePrefix(path string) string {
	vol := filepath.VolumeName(path)
	if vol != "" {
		return vol + string(filepath.Separator)
	}
	if filepath.IsAbs(path) {
		return string(filepath.Separator)
	}
	return ""
}
