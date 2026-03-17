package move

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxJSONLLineBytes = 16 * 1024 * 1024

// RewriteJSONLCWD rewrites session_meta/turn_context cwd prefixes in one session file.
func RewriteJSONLCWD(sessionFile, sourceRoot, targetRoot string) (bool, error) {
	in, err := os.Open(sessionFile)
	if err != nil {
		return false, err
	}
	defer in.Close()

	src := filepath.Clean(sourceRoot)
	dst := filepath.Clean(targetRoot)

	tmp, err := os.CreateTemp(filepath.Dir(sessionFile), filepath.Base(sessionFile)+".tmp-*")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	info, err := in.Stat()
	if err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Chmod(info.Mode()); err != nil {
		_ = tmp.Close()
		return false, err
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), maxJSONLLineBytes)
	writer := bufio.NewWriter(tmp)
	changed := false

	for scanner.Scan() {
		raw := scanner.Bytes()
		out := raw
		rowChanged, rewritten := rewriteJSONLLine(raw, src, dst)
		if rowChanged {
			changed = true
			out = rewritten
		}
		if _, err := writer.Write(out); err != nil {
			_ = tmp.Close()
			return false, err
		}
		if err := writer.WriteByte('\n'); err != nil {
			_ = tmp.Close()
			return false, err
		}
	}
	if err := scanner.Err(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := writer.Flush(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}

	if !changed {
		return false, nil
	}
	if err := os.Rename(tmpName, sessionFile); err != nil {
		return false, err
	}
	return true, nil
}

func rewriteJSONLLine(raw []byte, sourceRoot, targetRoot string) (bool, []byte) {
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return false, raw
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false, raw
	}
	eventType := jsonString(obj["type"])
	if eventType != "session_meta" && eventType != "turn_context" {
		return false, raw
	}

	payloadRaw, ok := obj["payload"]
	if !ok {
		return false, raw
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return false, raw
	}

	cwd := jsonString(payload["cwd"])
	newCWD, ok := remapPathPrefix(cwd, sourceRoot, targetRoot)
	if !ok {
		return false, raw
	}
	cwdRaw, err := json.Marshal(newCWD)
	if err != nil {
		return false, raw
	}
	payload["cwd"] = cwdRaw
	newPayload, err := json.Marshal(payload)
	if err != nil {
		return false, raw
	}
	obj["payload"] = newPayload
	encoded, err := json.Marshal(obj)
	if err != nil {
		return false, raw
	}
	return true, encoded
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func remapPathPrefix(path, sourceRoot, targetRoot string) (string, bool) {
	if !isUnderRoot(path, sourceRoot) {
		return "", false
	}
	rel, err := filepath.Rel(filepath.Clean(sourceRoot), filepath.Clean(path))
	if err != nil {
		return "", false
	}
	if rel == "." {
		return filepath.Clean(targetRoot), true
	}
	return filepath.Join(filepath.Clean(targetRoot), rel), true
}

// RewritePlanJSONL applies JSONL rewrites for each plan item session file.
func RewritePlanJSONL(plan Plan) error {
	for _, item := range plan.Items {
		if item.SessionFile == "" {
			return fmt.Errorf("missing session file for session %q", item.SessionID)
		}
		_, err := RewriteJSONLCWD(item.SessionFile, plan.SourceRoot, plan.TargetRoot)
		if err != nil {
			return fmt.Errorf("rewrite %s: %w", item.SessionFile, err)
		}
	}
	return nil
}
