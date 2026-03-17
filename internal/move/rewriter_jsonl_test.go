package move

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteJSONLCWDRewritesSessionMetaAndTurnContext(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "s.jsonl")
	content := strings.Join([]string{
		`{"type":"session_meta","payload":{"cwd":"/old/repo","session_id":"sid-1"}}`,
		`{"type":"turn_context","payload":{"cwd":"/old/repo/sub","model":"gpt-5"}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"keep"}}`,
	}, "\n")
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := RewriteJSONLCWD(f, "/old", "/new")
	if err != nil {
		t.Fatalf("RewriteJSONLCWD error: %v", err)
	}
	if !changed {
		t.Fatal("expected file to change")
	}

	lines := readLines(t, f)
	assertCWD(t, lines[0], "/new/repo")
	assertCWD(t, lines[1], "/new/repo/sub")
	if !strings.Contains(lines[2], `"event_msg"`) {
		t.Fatalf("expected non-cwd line unchanged semantically, got %q", lines[2])
	}
}

func TestRewriteJSONLCWDSkipsMalformedLine(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "s.jsonl")
	content := "not-json\n" + `{"type":"session_meta","payload":{"cwd":"/old/repo"}}`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := RewriteJSONLCWD(f, "/old", "/new")
	if err != nil {
		t.Fatalf("RewriteJSONLCWD error: %v", err)
	}
	if !changed {
		t.Fatal("expected rewrite to occur on valid line")
	}
	lines := readLines(t, f)
	if lines[0] != "not-json" {
		t.Fatalf("expected malformed line preserved, got %q", lines[0])
	}
	assertCWD(t, lines[1], "/new/repo")
}

func TestRewriteJSONLCWDBoundaryCheck(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "s.jsonl")
	content := `{"type":"session_meta","payload":{"cwd":"/oldx/repo"}}`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := RewriteJSONLCWD(f, "/old", "/new")
	if err != nil {
		t.Fatalf("RewriteJSONLCWD error: %v", err)
	}
	if changed {
		t.Fatal("expected no change for non-boundary prefix")
	}
	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != content {
		t.Fatalf("expected file unchanged, got %q", string(got))
	}
}

func assertCWD(t *testing.T, line, want string) {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		t.Fatalf("json unmarshal line: %v", err)
	}
	payload, ok := obj["payload"].(map[string]any)
	if !ok {
		t.Fatalf("missing payload in %q", line)
	}
	if got, _ := payload["cwd"].(string); got != want {
		t.Fatalf("unexpected cwd: got %q want %q", got, want)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}
