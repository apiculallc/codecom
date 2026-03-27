package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConversationMessages(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.jsonl")

	line1 := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello   world"}]}}`
	line2 := `{"type":"event_msg","payload":{"type":"assistant_message","message":" hi   there "}}`
	line3 := `{"type":"response_item","payload":{"type":"message","role":"system","content":[{"type":"text","text":"ignore"}]}}`
	line4 := `not-json`
	content := line1 + "\n" + line2 + "\n" + line3 + "\n" + line4 + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	msgs, err := parseConversationMessages(path)
	if err != nil {
		t.Fatalf("parseConversationMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Offset != 0 || msgs[0].Text != "hello world" {
		t.Fatalf("unexpected first message: %+v", msgs[0])
	}
	expectedSecondOffset := int64(len(line1) + 1)
	if msgs[1].Offset != expectedSecondOffset || msgs[1].Text != "hi there" {
		t.Fatalf("unexpected second message: %+v expectedOffset=%d", msgs[1], expectedSecondOffset)
	}
}

func TestParseConversationMessagesOversizedLineIsSkipped(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.jsonl")
	huge := strings.Repeat("a", maxConversationLineBytes+1)
	if err := os.WriteFile(path, []byte(huge), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	msgs, err := parseConversationMessages(path)
	if err != nil {
		t.Fatalf("parseConversationMessages: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no parsed messages, got %d", len(msgs))
	}
}
