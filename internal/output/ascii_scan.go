package output

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"codecom/internal/sessionindex"
)

const maxCWDWidth = 36

// WriteASCII writes a flat table of discovered sessions.
func WriteASCII(w io.Writer, res sessionindex.ScanResult) error {
	if _, err := fmt.Fprintln(w, "SESSION_ID  CWD  ORPHAN  LAST  TOTAL  FILE"); err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	for _, s := range res.Sessions {
		cwd := truncatePathLeft(s.EffectiveCWD(), maxCWDWidth, home)
		last := tokensOrNA(s.LastTokens)
		total := tokensOrNA(s.TotalTokens)
		orphan := "no"
		if s.Orphan {
			orphan = "yes"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", s.SessionID, cwd, orphan, last, total, s.SessionFile); err != nil {
			return err
		}
	}
	return nil
}

func tokensOrNA(v *int64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%d", *v)
}

func truncatePathLeft(path string, maxLen int, home string) string {
	if path == "" {
		return "n/a"
	}
	p := filepath.ToSlash(path)
	if home != "" {
		h := filepath.ToSlash(home)
		if p == h {
			p = "~"
		} else if strings.HasPrefix(p, h+"/") {
			p = "~/" + strings.TrimPrefix(p, h+"/")
		}
	}
	if len(p) <= maxLen {
		return p
	}
	if maxLen <= 2 {
		return p[len(p)-maxLen:]
	}
	return "…" + p[len(p)-(maxLen-1):]
}
