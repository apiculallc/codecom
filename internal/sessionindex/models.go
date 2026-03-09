package sessionindex

import (
	"regexp"
	"strings"
	"time"
)

var whitespaceRE = regexp.MustCompile(`\s+`)

// SessionRecord is one discovered Codex session file with extracted cwd metadata.
type SessionRecord struct {
	SessionID        string
	SessionFile      string
	SessionMetaCWD   string
	TurnContextCWD   []string
	FirstUserMessage string
	LastUserMessage  string
	LastActivity     time.Time
	UserMessageCount int
	Model            string
	Aborted          bool
	TotalTokens      *int64
	LastTokens       *int64
	WarningCount     int
	Orphan           bool
	SortTime         time.Time
}

// EffectiveCWD returns the best available cwd for display and orphan checks.
func (r SessionRecord) EffectiveCWD() string {
	if r.SessionMetaCWD != "" {
		return r.SessionMetaCWD
	}
	if n := len(r.TurnContextCWD); n > 0 {
		return r.TurnContextCWD[n-1]
	}
	return ""
}

// DisplayLabel returns the best available session summary for list displays.
func (r SessionRecord) DisplayLabel() string {
	if r.FirstUserMessage != "" {
		return normalizeDisplayText(r.FirstUserMessage)
	}
	if r.LastUserMessage != "" {
		return normalizeDisplayText(r.LastUserMessage)
	}
	return normalizeDisplayText(r.SessionID)
}

func normalizeDisplayText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return whitespaceRE.ReplaceAllString(s, " ")
}

type Warning struct {
	SessionFile string
	Line        int
	Message     string
}

type ScanResult struct {
	Sessions []SessionRecord
	Warnings []Warning
}
