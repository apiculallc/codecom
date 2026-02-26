package output

import (
	"encoding/json"
	"io"

	"codecom/internal/sessionindex"
)

type jsonlRow struct {
	SessionID     string `json:"session_id"`
	SessionFile   string `json:"session_file"`
	CWD           string `json:"cwd"`
	Orphan        bool   `json:"orphan"`
	TotalTokens   *int64 `json:"total_tokens,omitempty"`
	LastTokens    *int64 `json:"last_tokens,omitempty"`
	WarningsCount int    `json:"warnings_count"`
}

// WriteJSONL writes one JSON record per discovered session.
func WriteJSONL(w io.Writer, res sessionindex.ScanResult) error {
	enc := json.NewEncoder(w)
	for _, s := range res.Sessions {
		row := jsonlRow{
			SessionID:     s.SessionID,
			SessionFile:   s.SessionFile,
			CWD:           s.EffectiveCWD(),
			Orphan:        s.Orphan,
			TotalTokens:   s.TotalTokens,
			LastTokens:    s.LastTokens,
			WarningsCount: s.WarningCount,
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
