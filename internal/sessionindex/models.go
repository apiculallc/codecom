package sessionindex

import "time"

// SessionRecord is one discovered Codex session file with extracted cwd metadata.
type SessionRecord struct {
	SessionID      string
	SessionFile    string
	SessionMetaCWD string
	TurnContextCWD []string
	TotalTokens    *int64
	LastTokens     *int64
	WarningCount   int
	Orphan         bool
	SortTime       time.Time
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

type Warning struct {
	SessionFile string
	Line        int
	Message     string
}

type ScanResult struct {
	Sessions []SessionRecord
	Warnings []Warning
}
