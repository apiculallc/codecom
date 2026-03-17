package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotIfDirtyCreatesCommit(t *testing.T) {
	repo := initGitRepo(t)
	file := filepath.Join(repo, "a.txt")
	if err := os.WriteFile(file, []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repo, "add", "a.txt")
	runGitCmd(t, repo, "commit", "-m", "init")

	if err := os.WriteFile(file, []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	committed, err := SnapshotIfDirty(repo)
	if err != nil {
		t.Fatalf("SnapshotIfDirty error: %v", err)
	}
	if !committed {
		t.Fatal("expected dirty snapshot commit")
	}
	msg := strings.TrimSpace(runGitCmd(t, repo, "log", "-1", "--pretty=%s"))
	if msg != snapshotMessage {
		t.Fatalf("unexpected snapshot commit message: %q", msg)
	}
}

func TestCommitMoveCreatesTaggedCommit(t *testing.T) {
	repo := initGitRepo(t)
	sessionFile := filepath.Join(repo, "sessions", "2026", "03", "17", "s1.jsonl")
	sqliteFile := filepath.Join(repo, "state_5.sqlite")
	if err := os.MkdirAll(filepath.Dir(sessionFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionFile, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqliteFile, []byte("sqlite\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repo, "add", "-A")
	runGitCmd(t, repo, "commit", "-m", "init")

	if err := os.WriteFile(sessionFile, []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sqliteFile, []byte("sqlite2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	committed, err := CommitMove(repo, MoveCommitInput{
		SessionID:     "sid-1",
		SessionFile:   sessionFile,
		SQLiteFile:    sqliteFile,
		SourceRoot:    "/old",
		TargetRoot:    "/new",
		OldCWD:        "/old/repo",
		NewCWD:        "/new/repo",
		SQLiteUpdated: true,
	})
	if err != nil {
		t.Fatalf("CommitMove error: %v", err)
	}
	if !committed {
		t.Fatal("expected move commit")
	}

	body := runGitCmd(t, repo, "log", "-1", "--pretty=%B")
	if !strings.Contains(body, moveTrailer) {
		t.Fatalf("expected commit trailer %q in message %q", moveTrailer, body)
	}
	if !strings.Contains(body, "\"session_id\":\"sid-1\"") {
		t.Fatalf("expected json body in commit message: %q", body)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	repo := t.TempDir()
	runGitCmd(t, repo, "init")
	runGitCmd(t, repo, "config", "user.email", "codecom-test@example.com")
	runGitCmd(t, repo, "config", "user.name", "codecom-test")
	return repo
}

func runGitCmd(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, string(out))
	}
	return string(out)
}
