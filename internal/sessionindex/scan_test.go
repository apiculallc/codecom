package sessionindex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsSessionFilesAndSorts(t *testing.T) {
	root := t.TempDir()
	f1 := filepath.Join(root, "sessions", "2026", "02", "20", "a.jsonl")
	f2 := filepath.Join(root, "sessions", "2026", "02", "21", "b.jsonl")
	for _, f := range []string{f1, f2} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(f1, []byte("{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/scan-1\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/scan-1\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Scan(root)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(res.Sessions))
	}
	if got := filepath.Base(res.Sessions[0].SessionFile); got != "b.jsonl" {
		t.Fatalf("expected newest date first for same cwd, got %s", got)
	}
}
