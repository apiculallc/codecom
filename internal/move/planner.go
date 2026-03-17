package move

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"codecom/internal/sessionindex"
)

// Plan is a deterministic batch of session cwd remap operations.
type Plan struct {
	SourceRoot string
	TargetRoot string
	Items      []PlanItem
}

// PlanItem is one session remap computed from the selected source/target roots.
type PlanItem struct {
	SessionID   string
	SessionFile string
	OldCWD      string
	NewCWD      string
}

// BuildPlan computes remap targets for selected sessions, preserving relative suffix from source.
func BuildPlan(sourceRoot, targetRoot string, selected []sessionindex.SessionRecord) (Plan, error) {
	src := filepath.Clean(sourceRoot)
	dst := filepath.Clean(targetRoot)
	if src == "" || src == "." {
		return Plan{}, fmt.Errorf("invalid source root: %q", sourceRoot)
	}
	if dst == "" || dst == "." {
		return Plan{}, fmt.Errorf("invalid target root: %q", targetRoot)
	}

	items := make([]PlanItem, 0, len(selected))
	for _, rec := range selected {
		oldCWD := filepath.Clean(rec.EffectiveCWD())
		newCWD, err := remapPath(src, dst, oldCWD)
		if err != nil {
			newCWD = ""
		}
		items = append(items, PlanItem{
			SessionID:   rec.SessionID,
			SessionFile: rec.SessionFile,
			OldCWD:      oldCWD,
			NewCWD:      newCWD,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].SessionFile == items[j].SessionFile {
			return items[i].SessionID < items[j].SessionID
		}
		return items[i].SessionFile < items[j].SessionFile
	})

	return Plan{
		SourceRoot: src,
		TargetRoot: dst,
		Items:      items,
	}, nil
}

func remapPath(sourceRoot, targetRoot, cwd string) (string, error) {
	if !isUnderRoot(cwd, sourceRoot) {
		return "", fmt.Errorf("cwd %q is outside source root %q", cwd, sourceRoot)
	}
	rel, err := filepath.Rel(sourceRoot, cwd)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return targetRoot, nil
	}
	return filepath.Join(targetRoot, rel), nil
}

func isUnderRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." {
		return rel == "."
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
