package gitops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

func SnapshotIfDirty(repoRoot string, paths []string) (bool, error) {
	dirty, err := hasWorkingTreeDiff(repoRoot, paths)
	if err != nil {
		return false, err
	}
	if !dirty {
		return false, nil
	}
	pathspec, err := relPathspec(repoRoot, paths)
	if err != nil {
		return false, err
	}
	args := append([]string{"add", "--"}, pathspec...)
	if _, err := runGit(repoRoot, args...); err != nil {
		return false, err
	}
	changed, err := hasCachedDiff(repoRoot, pathspec)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	commitArgs := []string{"commit", "-m", snapshotMessage, "--"}
	commitArgs = append(commitArgs, pathspec...)
	if _, err := runGit(repoRoot, commitArgs...); err != nil {
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

	stagedDiff, err := hasCachedDiff(repoRoot, paths)
	if err != nil {
		return false, err
	}
	if !stagedDiff {
		return false, nil
	}

	bodyData, err := json.Marshal(map[string]any{
		"session_id":     in.SessionID,
		"session_file":   redactPath(in.SessionFile, repoRoot),
		"source_root":    redactPath(in.SourceRoot, repoRoot),
		"target_root":    redactPath(in.TargetRoot, repoRoot),
		"old_cwd":        redactPath(in.OldCWD, repoRoot),
		"new_cwd":        redactPath(in.NewCWD, repoRoot),
		"sqlite_updated": in.SQLiteUpdated,
	})
	if err != nil {
		return false, err
	}

	subject := fmt.Sprintf("codecom: cwd-change %s", in.SessionID)
	commitArgs := []string{"commit", "-m", subject, "-m", string(bodyData), "-m", moveTrailer, "--"}
	commitArgs = append(commitArgs, paths...)
	if _, err := runGit(repoRoot, commitArgs...); err != nil {
		return false, err
	}
	return true, nil
}

func hasCachedDiff(repoRoot string, paths []string) (bool, error) {
	args := []string{"-C", repoRoot, "diff", "--cached", "--quiet", "--"}
	args = append(args, paths...)
	cmd := exec.Command("git", args...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("git diff --cached --quiet --: %w", err)
	}
	return false, nil
}

func hasWorkingTreeDiff(repoRoot string, paths []string) (bool, error) {
	cmd := exec.Command("git", append([]string{"-C", repoRoot, "diff", "--quiet", "--"}, paths...)...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("git diff --quiet -- %s: %w", strings.Join(paths, " "), err)
	}

	cmd = exec.Command("git", append([]string{"-C", repoRoot, "diff", "--cached", "--quiet", "--"}, paths...)...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("git diff --cached --quiet -- %s: %w", strings.Join(paths, " "), err)
	}
	return false, nil
}

func relPathspec(repoRoot string, paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := filepath.Rel(repoRoot, p)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("path outside repo root: %q", p)
		}
		out = append(out, rel)
	}
	return out, nil
}

func redactPath(path, repoRoot string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	rel, err := filepath.Rel(repoRoot, path)
	if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return "./" + filepath.ToSlash(rel)
	}
	sum := sha256.Sum256([]byte(path))
	return "sha256:" + hex.EncodeToString(sum[:8])
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
