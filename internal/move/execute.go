package move

import (
	"fmt"
	"path/filepath"

	"codecom/internal/gitops"
	"codecom/internal/sessionindex"
)

type ExecuteResult struct {
	SnapshotCommitted bool
	FileCommits       int
}

// ExecutePlan performs validated JSONL+SQLite rewrites with git safety and commit policy.
func ExecutePlan(codexRoot string, plan Plan) (ExecuteResult, error) {
	if err := ValidatePlan(plan); err != nil {
		return ExecuteResult{}, err
	}
	if err := sessionindex.EnsureSQLiteThreadsReady(codexRoot); err != nil {
		return ExecuteResult{}, fmt.Errorf("sqlite preflight: %w", err)
	}

	snap, err := gitops.SnapshotIfDirty(codexRoot)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("git preflight snapshot: %w", err)
	}

	var commits int
	handled := make(map[string]struct{}, len(plan.Items))
	sqliteFile := filepath.Join(codexRoot, "state_5.sqlite")
	for _, item := range plan.Items {
		if _, ok := handled[item.SessionFile]; ok {
			continue
		}
		handled[item.SessionFile] = struct{}{}

		changed, err := RewriteJSONLCWD(item.SessionFile, plan.SourceRoot, plan.TargetRoot)
		if err != nil {
			return ExecuteResult{SnapshotCommitted: snap, FileCommits: commits}, fmt.Errorf("rewrite jsonl %s: %w", item.SessionFile, err)
		}
		if !changed {
			continue
		}

		if err := sessionindex.RewriteSQLiteThreadPaths(codexRoot, plan.SourceRoot, plan.TargetRoot, []string{item.SessionID}); err != nil {
			return ExecuteResult{SnapshotCommitted: snap, FileCommits: commits}, fmt.Errorf("rewrite sqlite for %s: %w", item.SessionID, err)
		}
		committed, err := gitops.CommitMove(codexRoot, gitops.MoveCommitInput{
			SessionID:     item.SessionID,
			SessionFile:   item.SessionFile,
			SQLiteFile:    sqliteFile,
			SourceRoot:    plan.SourceRoot,
			TargetRoot:    plan.TargetRoot,
			OldCWD:        item.OldCWD,
			NewCWD:        item.NewCWD,
			SQLiteUpdated: true,
		})
		if err != nil {
			return ExecuteResult{SnapshotCommitted: snap, FileCommits: commits}, fmt.Errorf("commit move for %s: %w", item.SessionID, err)
		}
		if committed {
			commits++
		}
	}

	return ExecuteResult{
		SnapshotCommitted: snap,
		FileCommits:       commits,
	}, nil
}
