package output

import (
	"bytes"
	"strings"
	"testing"

	"codecom/internal/sessionindex"
)

func TestWriteASCII(t *testing.T) {
	res := sessionindex.ScanResult{Sessions: []sessionindex.SessionRecord{{
		SessionID:      "sid-1",
		SessionFile:    "/tmp/a.jsonl",
		SessionMetaCWD: "/a/very/long/path/that/should/be/truncated/by/the/formatter",
		Orphan:         true,
	}}}

	var out bytes.Buffer
	if err := WriteASCII(&out, res); err != nil {
		t.Fatalf("WriteASCII error: %v", err)
	}

	txt := out.String()
	if !strings.Contains(txt, "SESSION_ID  CWD  ORPHAN  LAST  TOTAL  FILE") {
		t.Fatalf("missing header: %q", txt)
	}
	if !strings.Contains(txt, "sid-1") || !strings.Contains(txt, "yes") || !strings.Contains(txt, "n/a") {
		t.Fatalf("missing row values: %q", txt)
	}
	if !strings.Contains(txt, "…") {
		t.Fatalf("expected left-truncated path marker: %q", txt)
	}
}

func TestTruncatePathLeft(t *testing.T) {
	got := truncatePathLeft("/home/user/dev/java/app1", 12, "/home/user")
	if got != "…v/java/app1" {
		t.Fatalf("unexpected truncation: %q", got)
	}
}
