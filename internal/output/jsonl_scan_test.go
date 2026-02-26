package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"codecom/internal/sessionindex"
)

func TestWriteJSONL(t *testing.T) {
	total := int64(101)
	last := int64(9)
	res := sessionindex.ScanResult{Sessions: []sessionindex.SessionRecord{{
		SessionID:      "sid-1",
		SessionFile:    "/tmp/a.jsonl",
		SessionMetaCWD: "/tmp/repo",
		Orphan:         false,
		TotalTokens:    &total,
		LastTokens:     &last,
		WarningCount:   2,
	}}}

	var out bytes.Buffer
	if err := WriteJSONL(&out, res); err != nil {
		t.Fatalf("WriteJSONL error: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if got["session_id"] != "sid-1" {
		t.Fatalf("unexpected session_id: %#v", got["session_id"])
	}
	if got["cwd"] != "/tmp/repo" {
		t.Fatalf("unexpected cwd: %#v", got["cwd"])
	}
	if got["warnings_count"].(float64) != 2 {
		t.Fatalf("unexpected warnings_count: %#v", got["warnings_count"])
	}
}
