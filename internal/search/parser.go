package search

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type messageLine struct {
	Offset int64
	Text   string
}

type conversationEnvelope struct {
	Type      string          `json:"type"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

type responsePayload struct {
	Type    string            `json:"type"`
	Role    string            `json:"role"`
	Content []responseContent `json:"content"`
}

type responseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type eventMessagePayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func parseConversationMessages(path string) ([]messageLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	out := make([]messageLine, 0, 128)
	var offset int64
	for {
		raw, err := reader.ReadBytes('\n')
		if len(raw) > 0 {
			if txt, ok := parseConversationLine(raw); ok {
				out = append(out, messageLine{Offset: offset, Text: txt})
			}
			offset += int64(len(raw))
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			break
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}
	return out, nil
}

func parseConversationLine(raw []byte) (string, bool) {
	var env conversationEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", false
	}

	lineType := env.Type
	if lineType == "" {
		lineType = env.EventType
	}

	switch lineType {
	case "response_item":
		var p responsePayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return "", false
		}
		if p.Type != "message" {
			return "", false
		}
		if p.Role != "user" && p.Role != "assistant" {
			return "", false
		}
		parts := make([]string, 0, len(p.Content))
		for _, c := range p.Content {
			switch c.Type {
			case "input_text", "output_text", "text":
				t := normalizeSearchText(c.Text)
				if t != "" {
					parts = append(parts, t)
				}
			}
		}
		if len(parts) == 0 {
			return "", false
		}
		return strings.Join(parts, " "), true
	case "event_msg":
		var p eventMessagePayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return "", false
		}
		switch p.Type {
		case "user_message", "assistant_message", "agent_message":
			t := normalizeSearchText(p.Message)
			if t == "" {
				return "", false
			}
			return t, true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

func normalizeSearchText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
