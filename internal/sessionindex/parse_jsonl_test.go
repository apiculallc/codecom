package sessionindex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSessionFileExtractsFieldsAndWarnings(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "s.jsonl")
	content := "" +
		"{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/a\",\"session_id\":\"sid-1\"}}\n" +
		"{\"type\":\"turn_context\",\"payload\":{\"cwd\":\"/tmp/b\"}}\n" +
		"not-json\n" +
		"{\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"total_token_usage\":100,\"last_token_usage\":10}}\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rec, warnings, err := parseSessionFile(f)
	if err != nil {
		t.Fatalf("parseSessionFile returned error: %v", err)
	}
	if rec.SessionID != "sid-1" {
		t.Fatalf("unexpected session id: %q", rec.SessionID)
	}
	if rec.SessionMetaCWD != "/tmp/a" {
		t.Fatalf("unexpected session meta cwd: %q", rec.SessionMetaCWD)
	}
	if len(rec.TurnContextCWD) != 1 || rec.TurnContextCWD[0] != "/tmp/b" {
		t.Fatalf("unexpected turn cwd list: %#v", rec.TurnContextCWD)
	}
	if rec.TotalTokens == nil || *rec.TotalTokens != 100 {
		t.Fatalf("unexpected total tokens: %#v", rec.TotalTokens)
	}
	if rec.LastTokens == nil || *rec.LastTokens != 10 {
		t.Fatalf("unexpected last tokens: %#v", rec.LastTokens)
	}
	if got := len(warnings); got != 1 {
		t.Fatalf("expected 1 warning, got %d", got)
	}
	if rec.WarningCount != 1 {
		t.Fatalf("expected warning count 1, got %d", rec.WarningCount)
	}
}

func TestParseSessionFileFallsBackSessionIDFromFilename(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "rollout-abc.jsonl")
	if err := os.WriteFile(f, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec, _, err := parseSessionFile(f)
	if err != nil {
		t.Fatalf("parseSessionFile returned error: %v", err)
	}
	if rec.SessionID != "rollout-abc" {
		t.Fatalf("unexpected fallback session id: %q", rec.SessionID)
	}
}
