package move

import (
	"path/filepath"
	"testing"
)

func TestValidatePlanAggregatesErrors(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "src")
	target := filepath.Join(root, "dst")
	validTarget := filepath.Join(target, "ok")
	missingTarget := filepath.Join(target, "missing")
	if err := mkdirAll(source, validTarget); err != nil {
		t.Fatal(err)
	}

	plan := Plan{
		SourceRoot: source,
		TargetRoot: target,
		Items: []PlanItem{
			{SessionID: "", SessionFile: "", OldCWD: filepath.Join(source, "ok"), NewCWD: validTarget},
			{SessionID: "sid-out", SessionFile: "/tmp/out.jsonl", OldCWD: filepath.Join(root, "outside"), NewCWD: validTarget},
			{SessionID: "sid-missing", SessionFile: "/tmp/missing.jsonl", OldCWD: filepath.Join(source, "missing"), NewCWD: missingTarget},
		},
	}

	err := ValidatePlan(plan)
	if err == nil {
		t.Fatal("expected validation error")
	}
	verr, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if len(verr.Items) < 4 {
		t.Fatalf("expected multiple aggregated validation errors, got %#v", verr.Items)
	}
}

func TestValidatePlanSuccess(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "src")
	target := filepath.Join(root, "dst")
	oldCWD := filepath.Join(source, "repo", "a")
	newCWD := filepath.Join(target, "repo", "a")
	if err := mkdirAll(oldCWD, newCWD); err != nil {
		t.Fatal(err)
	}
	plan := Plan{
		SourceRoot: source,
		TargetRoot: target,
		Items: []PlanItem{
			{
				SessionID:   "sid-1",
				SessionFile: "/tmp/sid-1.jsonl",
				OldCWD:      oldCWD,
				NewCWD:      newCWD,
			},
		},
	}
	if err := ValidatePlan(plan); err != nil {
		t.Fatalf("ValidatePlan error: %v", err)
	}
}
