package move

import (
	"path/filepath"
	"testing"

	"codecom/internal/sessionindex"
)

func TestBuildPlanDeterministicAndMapped(t *testing.T) {
	selected := []sessionindex.SessionRecord{
		{SessionID: "sid-2", SessionFile: "/tmp/z.jsonl", SessionMetaCWD: "/src/repo/a"},
		{SessionID: "sid-1", SessionFile: "/tmp/a.jsonl", SessionMetaCWD: "/src/repo/b"},
	}
	plan, err := BuildPlan("/src", "/dst", selected)
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(plan.Items))
	}
	if plan.Items[0].SessionID != "sid-1" {
		t.Fatalf("expected deterministic sort by file/session, got %+v", plan.Items)
	}
	if got := plan.Items[0].NewCWD; got != filepath.Clean("/dst/repo/b") {
		t.Fatalf("unexpected mapped cwd: %q", got)
	}
	if got := plan.Items[1].NewCWD; got != filepath.Clean("/dst/repo/a") {
		t.Fatalf("unexpected mapped cwd: %q", got)
	}
}

func TestBuildPlanHandlesOutsideSourceByLeavingNewCWDUnset(t *testing.T) {
	selected := []sessionindex.SessionRecord{
		{SessionID: "sid-1", SessionFile: "/tmp/a.jsonl", SessionMetaCWD: "/other/repo"},
	}
	plan, err := BuildPlan("/src", "/dst", selected)
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(plan.Items))
	}
	if plan.Items[0].NewCWD != "" {
		t.Fatalf("expected empty NewCWD for out-of-root path, got %q", plan.Items[0].NewCWD)
	}
}
