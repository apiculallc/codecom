package sessionindex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	CWD       string `json:"cwd"`
	SessionID string `json:"session_id"`
	ID        string `json:"id"`
}

type turnContextPayload struct {
	CWD string `json:"cwd"`
}

type tokenPayload struct {
	Type            string `json:"type"`
	TotalTokenUsage *int64 `json:"total_token_usage"`
	LastTokenUsage  *int64 `json:"last_token_usage"`
}

func parseSessionFile(path string) (SessionRecord, []Warning, error) {
	f, err := os.Open(path)
	if err != nil {
		return SessionRecord{}, nil, err
	}
	defer f.Close()

	rec := SessionRecord{SessionFile: path}
	warnings := make([]Warning, 0)

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var env envelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			warnings = append(warnings, Warning{SessionFile: path, Line: lineNo, Message: fmt.Sprintf("malformed json line: %v", err)})
			continue
		}

		switch env.Type {
		case "session_meta":
			var p sessionMetaPayload
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				warnings = append(warnings, Warning{SessionFile: path, Line: lineNo, Message: fmt.Sprintf("bad session_meta payload: %v", err)})
				continue
			}
			rec.SessionMetaCWD = p.CWD
			if rec.SessionID == "" {
				rec.SessionID = firstNonEmpty(p.SessionID, p.ID)
			}
		case "turn_context":
			var p turnContextPayload
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				warnings = append(warnings, Warning{SessionFile: path, Line: lineNo, Message: fmt.Sprintf("bad turn_context payload: %v", err)})
				continue
			}
			if p.CWD != "" {
				rec.TurnContextCWD = append(rec.TurnContextCWD, p.CWD)
			}
		case "event_msg":
			var p tokenPayload
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				warnings = append(warnings, Warning{SessionFile: path, Line: lineNo, Message: fmt.Sprintf("bad event_msg payload: %v", err)})
				continue
			}
			if p.Type == "token_count" {
				rec.TotalTokens = p.TotalTokenUsage
				rec.LastTokens = p.LastTokenUsage
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return SessionRecord{}, warnings, err
	}

	if rec.SessionID == "" {
		rec.SessionID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	rec.WarningCount = len(warnings)
	return rec, warnings, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
