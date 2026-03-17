package move

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codecom/internal/sessionindex"
	_ "modernc.org/sqlite"
)

func TestExecutePlanRewritesAndCommitsPerSessionFile(t *testing.T) {
	repo := initGitRepo(t)
	sourceRoot := filepath.Join(repo, "workspace", "old")
	targetRoot := filepath.Join(repo, "workspace", "new")
	oldCWD := filepath.Join(sourceRoot, "proj")
	newCWD := filepath.Join(targetRoot, "proj")
	sessionFile := filepath.Join(repo, "sessions", "2026", "03", "17", "s1.jsonl")
	sqlitePath := filepath.Join(repo, "state_5.sqlite")

	mkdirAllT(t, oldCWD, newCWD, filepath.Dir(sessionFile))
	writeFileT(t, sessionFile, `{"type":"session_meta","payload":{"cwd":"`+oldCWD+`","session_id":"sid-1"}}`+"\n")
	writeSQLiteThreads(t, sqlitePath, []threadRow{{
		ID:          "sid-1",
		CWD:         oldCWD,
		RolloutPath: filepath.Join(sourceRoot, ".codex", "sessions", "s1.jsonl"),
	}})
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "init")

	selected := []sessionindex.SessionRecord{{
		SessionID:      "sid-1",
		SessionFile:    sessionFile,
		SessionMetaCWD: oldCWD,
	}}
	plan, err := BuildPlan(sourceRoot, targetRoot, selected)
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	got, err := ExecutePlan(repo, plan)
	if err != nil {
		t.Fatalf("ExecutePlan error: %v", err)
	}
	if got.SnapshotCommitted {
		t.Fatal("expected no snapshot commit for clean repo")
	}
	if got.FileCommits != 1 {
		t.Fatalf("expected one file commit, got %d", got.FileCommits)
	}

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), newCWD) {
		t.Fatalf("expected rewritten session file with new cwd %q, got %q", newCWD, string(data))
	}
	assertSQLiteCWD(t, sqlitePath, "sid-1", newCWD)

	lastSubject := strings.TrimSpace(runGit(t, repo, "log", "-1", "--pretty=%s"))
	if lastSubject != "codecom: cwd-change sid-1" {
		t.Fatalf("unexpected commit subject: %q", lastSubject)
	}
	lastBody := runGit(t, repo, "log", "-1", "--pretty=%B")
	if !strings.Contains(lastBody, "Codecom-Action: cwd-change") {
		t.Fatalf("expected move trailer in commit body: %q", lastBody)
	}
}

func TestExecutePlanCreatesSnapshotWhenDirty(t *testing.T) {
	repo := initGitRepo(t)
	sourceRoot := filepath.Join(repo, "workspace", "old")
	targetRoot := filepath.Join(repo, "workspace", "new")
	oldCWD := filepath.Join(sourceRoot, "proj")
	sessionFile := filepath.Join(repo, "sessions", "2026", "03", "17", "s1.jsonl")
	sqlitePath := filepath.Join(repo, "state_5.sqlite")

	mkdirAllT(t, oldCWD, filepath.Join(targetRoot, "proj"), filepath.Dir(sessionFile))
	writeFileT(t, sessionFile, `{"type":"session_meta","payload":{"cwd":"`+oldCWD+`","session_id":"sid-1"}}`+"\n")
	writeSQLiteThreads(t, sqlitePath, []threadRow{{ID: "sid-1", CWD: oldCWD}})
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "init")

	// Pre-existing unrelated dirty change.
	writeFileT(t, filepath.Join(repo, "dirty.txt"), "dirty\n")

	plan, err := BuildPlan(sourceRoot, targetRoot, []sessionindex.SessionRecord{{
		SessionID:      "sid-1",
		SessionFile:    sessionFile,
		SessionMetaCWD: oldCWD,
	}})
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	got, err := ExecutePlan(repo, plan)
	if err != nil {
		t.Fatalf("ExecutePlan error: %v", err)
	}
	if !got.SnapshotCommitted {
		t.Fatal("expected dirty snapshot commit")
	}
	second := strings.TrimSpace(runGit(t, repo, "log", "-2", "--pretty=%s"))
	if !strings.Contains(second, "codecom: snapshot pre-existing local changes") {
		t.Fatalf("expected snapshot commit in last two messages, got %q", second)
	}
}

func TestExecutePlanValidationFailureDoesNotWrite(t *testing.T) {
	repo := initGitRepo(t)
	sourceRoot := filepath.Join(repo, "workspace", "old")
	targetRoot := filepath.Join(repo, "workspace", "new")
	oldCWD := filepath.Join(sourceRoot, "proj")
	// Intentionally do not create mapped target directory under targetRoot.
	sessionFile := filepath.Join(repo, "sessions", "2026", "03", "17", "s1.jsonl")
	sqlitePath := filepath.Join(repo, "state_5.sqlite")

	mkdirAllT(t, oldCWD, filepath.Dir(sessionFile))
	writeFileT(t, sessionFile, `{"type":"session_meta","payload":{"cwd":"`+oldCWD+`","session_id":"sid-1"}}`+"\n")
	writeSQLiteThreads(t, sqlitePath, []threadRow{{ID: "sid-1", CWD: oldCWD}})
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-m", "init")

	beforeJSON, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	beforeCWD := sqliteCWD(t, sqlitePath, "sid-1")
	beforeHead := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	plan := Plan{
		SourceRoot: sourceRoot,
		TargetRoot: targetRoot,
		Items: []PlanItem{{
			SessionID:   "sid-1",
			SessionFile: sessionFile,
			OldCWD:      oldCWD,
			NewCWD:      filepath.Join(targetRoot, "proj"),
		}},
	}
	_, err = ExecutePlan(repo, plan)
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if _, ok := err.(*ValidationErrors); !ok {
		t.Fatalf("expected ValidationErrors, got %T (%v)", err, err)
	}

	afterJSON, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterJSON) != string(beforeJSON) {
		t.Fatalf("session file should remain unchanged, before=%q after=%q", string(beforeJSON), string(afterJSON))
	}
	afterCWD := sqliteCWD(t, sqlitePath, "sid-1")
	if afterCWD != beforeCWD {
		t.Fatalf("sqlite cwd changed unexpectedly: before=%q after=%q", beforeCWD, afterCWD)
	}
	afterHead := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	if afterHead != beforeHead {
		t.Fatalf("git head changed unexpectedly: before=%s after=%s", beforeHead, afterHead)
	}
}

type threadRow struct {
	ID          string
	CWD         string
	RolloutPath string
}

func writeSQLiteThreads(t *testing.T, dbPath string, rows []threadRow) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE threads (id TEXT PRIMARY KEY, cwd TEXT, rollout_path TEXT);`); err != nil {
		t.Fatalf("create threads: %v", err)
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO threads(id, cwd, rollout_path) VALUES(?, ?, ?)`, row.ID, row.CWD, row.RolloutPath); err != nil {
			t.Fatalf("insert thread %s: %v", row.ID, err)
		}
	}
}

func assertSQLiteCWD(t *testing.T, dbPath, id, want string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	var got string
	if err := db.QueryRow(`SELECT cwd FROM threads WHERE id = ?`, id).Scan(&got); err != nil {
		t.Fatalf("query sqlite cwd: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected sqlite cwd: got %q want %q", got, want)
	}
}

func sqliteCWD(t *testing.T, dbPath, id string) string {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	var got string
	if err := db.QueryRow(`SELECT cwd FROM threads WHERE id = ?`, id).Scan(&got); err != nil {
		t.Fatalf("query sqlite cwd: %v", err)
	}
	return got
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "codecom-test@example.com")
	runGit(t, repo, "config", "user.name", "codecom-test")
	return repo
}

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, string(out))
	}
	return string(out)
}

func mkdirAllT(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
}

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
