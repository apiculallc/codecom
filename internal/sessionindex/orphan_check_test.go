package sessionindex

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultOrphanWorkersBounded(t *testing.T) {
	got := DefaultOrphanWorkers()
	if got < 1 || got > 32 {
		t.Fatalf("worker count out of bounds: %d", got)
	}
	expected := runtime.NumCPU() * 2
	if expected > 32 {
		expected = 32
	}
	if expected < 1 {
		expected = 1
	}
	if got != expected {
		t.Fatalf("expected %d workers, got %d", expected, got)
	}
}

func TestApplyOrphanStatus(t *testing.T) {
	existing := t.TempDir()
	nonexistent := filepath.Join(t.TempDir(), "missing")

	records := []SessionRecord{
		{SessionMetaCWD: existing},
		{SessionMetaCWD: nonexistent},
		{},
	}
	ApplyOrphanStatus(records, 2)

	if records[0].Orphan {
		t.Fatal("existing path should not be orphan")
	}
	if !records[1].Orphan {
		t.Fatal("missing path should be orphan")
	}
	if !records[2].Orphan {
		t.Fatal("empty cwd should be orphan")
	}
}
