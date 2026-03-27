package search

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"codecom/internal/sessionindex"
)

func TestBuildSQLiteIndexAndSearch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	s1 := filepath.Join(tmp, "s1.jsonl")
	s2 := filepath.Join(tmp, "s2.jsonl")
	s3 := filepath.Join(tmp, "s3.jsonl")

	s1line1 := `{"type":"event_msg","payload":{"type":"user_message","message":"Alpha beta"}}`
	s1line2 := `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"gamma phrase"}]}}`
	mustWriteJSONL(t, s1, []string{s1line1, s1line2})
	mustWriteJSONL(t, s2, []string{
		`{"type":"event_msg","payload":{"type":"user_message","message":"alpha only"}}`,
		`{"type":"event_msg","payload":{"type":"assistant_message","message":"other"}}`,
	})
	mustWriteJSONL(t, s3, []string{
		`{"type":"event_msg","payload":{"type":"assistant_message","message":"zeta"}}`,
	})

	records := []sessionindex.SessionRecord{
		{SessionID: "sid-1", SessionFile: s1, SessionMetaCWD: "/repo/a"},
		{SessionID: "sid-2", SessionFile: s2, SessionMetaCWD: "/repo/b"},
		{SessionID: "sid-3", SessionFile: s3, SessionMetaCWD: "/repo/c"},
	}

	backend, err := BuildSQLiteIndex("/unused", records)
	if err != nil {
		t.Fatalf("BuildSQLiteIndex: %v", err)
	}
	defer backend.Close()

	res, err := backend.Search(`ALPHA "gamma phrase"`)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if _, ok := res.SessionIDs["sid-1"]; !ok || len(res.SessionIDs) != 1 {
		t.Fatalf("unexpected session ids: %#v", res.SessionIDs)
	}
	if _, ok := res.FolderPaths["/repo/a"]; !ok || len(res.FolderPaths) != 1 {
		t.Fatalf("unexpected folder paths: %#v", res.FolderPaths)
	}
	if got := res.OffsetsBySessionID["sid-1"]; !reflect.DeepEqual(got, []int64{0, int64(len(s1line1) + 1)}) {
		t.Fatalf("unexpected offsets for sid-1: %v", got)
	}

	res2, err := backend.Search("alpha")
	if err != nil {
		t.Fatalf("Search alpha: %v", err)
	}
	if len(res2.SessionIDs) != 2 {
		t.Fatalf("expected 2 sessions for alpha, got %#v", res2.SessionIDs)
	}

	res3, err := backend.Search("alpha missing")
	if err != nil {
		t.Fatalf("Search alpha missing: %v", err)
	}
	if len(res3.SessionIDs) != 0 || len(res3.FolderPaths) != 0 || len(res3.OffsetsBySessionID) != 0 {
		t.Fatalf("expected empty result, got %#v", res3)
	}
}

func mustWriteJSONL(t *testing.T, path string, lines []string) {
	t.Helper()
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
