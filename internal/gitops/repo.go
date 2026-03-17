package gitops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	snapshotMessage = "codecom: snapshot pre-existing local changes"
	moveTrailer     = "Codecom-Action: cwd-change"
)

type MoveCommitInput struct {
	SessionID     string
	SessionFile   string
	SQLiteFile    string
	SourceRoot    string
	TargetRoot    string
	OldCWD        string
	NewCWD        string
	SQLiteUpdated bool
}

func IsDirty(repoRoot string) (bool, error) {
	out, err := runGit(repoRoot, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func SnapshotIfDirty(repoRoot string) (bool, error) {
	dirty, err := IsDirty(repoRoot)
	if err != nil {
		return false, err
	}
	if !dirty {
		return false, nil
	}
	if _, err := runGit(repoRoot, "add", "-A"); err != nil {
		return false, err
	}
	if _, err := runGit(repoRoot, "commit", "-m", snapshotMessage); err != nil {
		return false, err
	}
	return true, nil
}

func CommitMove(repoRoot string, in MoveCommitInput) (bool, error) {
	paths := make([]string, 0, 2)
	sessionRel, err := filepath.Rel(repoRoot, in.SessionFile)
	if err != nil || strings.HasPrefix(sessionRel, "..") {
		return false, fmt.Errorf("session file outside repo root: %q", in.SessionFile)
	}
	paths = append(paths, sessionRel)
	if in.SQLiteFile != "" {
		sqliteRel, err := filepath.Rel(repoRoot, in.SQLiteFile)
		if err != nil || strings.HasPrefix(sqliteRel, "..") {
			return false, fmt.Errorf("sqlite file outside repo root: %q", in.SQLiteFile)
		}
		paths = append(paths, sqliteRel)
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := runGit(repoRoot, args...); err != nil {
		return false, err
	}

	stagedDiff, err := hasCachedDiff(repoRoot)
	if err != nil {
		return false, err
	}
	if !stagedDiff {
		return false, nil
	}

	bodyData, err := json.Marshal(map[string]any{
		"session_id":     in.SessionID,
		"session_file":   in.SessionFile,
		"source_root":    in.SourceRoot,
		"target_root":    in.TargetRoot,
		"old_cwd":        in.OldCWD,
		"new_cwd":        in.NewCWD,
		"sqlite_updated": in.SQLiteUpdated,
	})
	if err != nil {
		return false, err
	}

	subject := fmt.Sprintf("codecom: cwd-change %s", in.SessionID)
	if _, err := runGit(repoRoot, "commit", "-m", subject, "-m", string(bodyData), "-m", moveTrailer); err != nil {
		return false, err
	}
	return true, nil
}

func hasCachedDiff(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--cached", "--quiet", "--")
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("git diff --cached --quiet --: %w", err)
	}
	return false, nil
}

func runGit(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}
