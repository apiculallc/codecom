package move

import (
	"fmt"
	"sort"

	"codecom/internal/sessionindex"
)

// RewritePlanSQLite rewrites sqlite threads rows for selected session ids in a plan.
func RewritePlanSQLite(codexRoot string, plan Plan) error {
	if err := sessionindex.EnsureSQLiteThreadsReady(codexRoot); err != nil {
		return fmt.Errorf("sqlite preflight: %w", err)
	}
	ids := planSessionIDs(plan)
	if len(ids) == 0 {
		return nil
	}
	if err := sessionindex.RewriteSQLiteThreadPaths(codexRoot, plan.SourceRoot, plan.TargetRoot, ids); err != nil {
		return fmt.Errorf("sqlite rewrite: %w", err)
	}
	return nil
}

func planSessionIDs(plan Plan) []string {
	seen := make(map[string]struct{}, len(plan.Items))
	for _, item := range plan.Items {
		if item.SessionID == "" {
			continue
		}
		seen[item.SessionID] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
