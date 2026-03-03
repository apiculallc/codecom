package sessionindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSessionFileExtractsFieldsAndWarnings(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "s.jsonl")
	content := "" +
		"{\"timestamp\":\"2026-03-03T10:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/a\",\"session_id\":\"sid-1\"}}\n" +
		"{\"timestamp\":\"2026-03-03T10:01:00Z\",\"type\":\"turn_context\",\"payload\":{\"cwd\":\"/tmp/b\",\"model\":\"gpt-5\"}}\n" +
		"{\"timestamp\":\"2026-03-03T10:02:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"user_message\",\"message\":\"first prompt\"}}\n" +
		"not-json\n" +
		"{\"timestamp\":\"2026-03-03T10:03:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"user_message\",\"message\":\"last prompt\"}}\n" +
		"{\"timestamp\":\"2026-03-03T10:04:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"turn_aborted\"}}\n" +
		"{\"timestamp\":\"2026-03-03T10:05:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"total_token_usage\":100,\"last_token_usage\":10}}\n"
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
	if rec.Model != "gpt-5" {
		t.Fatalf("unexpected model: %q", rec.Model)
	}
	if rec.FirstUserMessage != "first prompt" || rec.LastUserMessage != "last prompt" {
		t.Fatalf("unexpected user messages: first=%q last=%q", rec.FirstUserMessage, rec.LastUserMessage)
	}
	if rec.UserMessageCount != 2 {
		t.Fatalf("unexpected user message count: %d", rec.UserMessageCount)
	}
	if !rec.Aborted {
		t.Fatal("expected aborted=true")
	}
	wantActivity := time.Date(2026, 3, 3, 10, 5, 0, 0, time.UTC)
	if !rec.LastActivity.Equal(wantActivity) {
		t.Fatalf("unexpected last activity: %s", rec.LastActivity)
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

func TestParseSessionFileHandlesLargeLines(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "large.jsonl")
	longCWD := "/tmp/" + strings.Repeat("a", 70*1024)
	line := "{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"" + longCWD + "\",\"session_id\":\"sid-large\"}}\n"
	if err := os.WriteFile(f, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	rec, warnings, err := parseSessionFile(f)
	if err != nil {
		t.Fatalf("parseSessionFile returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %d", len(warnings))
	}
	if rec.SessionID != "sid-large" {
		t.Fatalf("unexpected session id: %q", rec.SessionID)
	}
	if rec.SessionMetaCWD != longCWD {
		t.Fatalf("unexpected cwd length: got %d want %d", len(rec.SessionMetaCWD), len(longCWD))
	}
}

func TestSessionRecordDisplayLabelPrefersFirstUserMessage(t *testing.T) {
	rec := SessionRecord{SessionID: "sid-1", FirstUserMessage: "first", LastUserMessage: "last"}
	if got := rec.DisplayLabel(); got != "first" {
		t.Fatalf("unexpected display label: %q", got)
	}
	rec = SessionRecord{SessionID: "sid-1", LastUserMessage: "last"}
	if got := rec.DisplayLabel(); got != "last" {
		t.Fatalf("unexpected display label fallback: %q", got)
	}
	rec = SessionRecord{SessionID: "sid-1"}
	if got := rec.DisplayLabel(); got != "sid-1" {
		t.Fatalf("unexpected display label final fallback: %q", got)
	}
}
