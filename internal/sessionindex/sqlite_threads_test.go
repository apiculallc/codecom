package sessionindex

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestReadSQLiteThreadsDB(t *testing.T) {
	db := openMemorySQLite(t)
	mustExecSQL(t, db, `
CREATE TABLE threads (
  id TEXT PRIMARY KEY,
  cwd TEXT,
  rollout_path TEXT
);
INSERT INTO threads (id, cwd, rollout_path) VALUES
  ('sid-1', '/repo/a', '/sessions/a.jsonl'),
  ('sid-2', '/repo/b', NULL);
`)

	rows, err := readSQLiteThreadsDB(db)
	if err != nil {
		t.Fatalf("readSQLiteThreadsDB error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows["sid-1"].CWD != "/repo/a" {
		t.Fatalf("sid-1 cwd mismatch: %q", rows["sid-1"].CWD)
	}
	if rows["sid-2"].RolloutPath != "" {
		t.Fatalf("sid-2 rollout path should be empty for NULL, got %q", rows["sid-2"].RolloutPath)
	}
}

func TestRewriteSQLiteThreadPathsDB(t *testing.T) {
	db := openMemorySQLite(t)
	mustExecSQL(t, db, `
CREATE TABLE threads (
  id TEXT PRIMARY KEY,
  cwd TEXT,
  rollout_path TEXT
);
INSERT INTO threads (id, cwd, rollout_path) VALUES
  ('sid-1', '/home/bofh/repo-a', '/home/bofh/.codex/sessions/a.jsonl'),
  ('sid-2', '/home/bofh/repo-b', '/home/bofh/.codex/sessions/b.jsonl');
`)

	if err := rewriteSQLiteThreadPathsDB(db, "/home/bofh", "/home/devel", []string{"sid-1"}); err != nil {
		t.Fatalf("rewriteSQLiteThreadPathsDB error: %v", err)
	}

	rows, err := readSQLiteThreadsDB(db)
	if err != nil {
		t.Fatalf("readSQLiteThreadsDB error: %v", err)
	}
	if rows["sid-1"].CWD != "/home/devel/repo-a" {
		t.Fatalf("sid-1 cwd mismatch: %q", rows["sid-1"].CWD)
	}
	if rows["sid-1"].RolloutPath != "/home/devel/.codex/sessions/a.jsonl" {
		t.Fatalf("sid-1 rollout mismatch: %q", rows["sid-1"].RolloutPath)
	}
	if rows["sid-2"].CWD != "/home/bofh/repo-b" {
		t.Fatalf("sid-2 should stay unchanged, got %q", rows["sid-2"].CWD)
	}
}

func TestEnsureSQLiteThreadsReadyDB(t *testing.T) {
	db := openMemorySQLite(t)
	mustExecSQL(t, db, `
CREATE TABLE threads (
  id TEXT PRIMARY KEY,
  cwd TEXT,
  rollout_path TEXT
);
`)
	if err := ensureSQLiteThreadsReadyDB(db); err != nil {
		t.Fatalf("ensureSQLiteThreadsReadyDB error: %v", err)
	}
}

func TestEnsureSQLiteThreadsReadyDBMissingTable(t *testing.T) {
	db := openMemorySQLite(t)
	if err := ensureSQLiteThreadsReadyDB(db); err == nil {
		t.Fatal("expected error when threads table is missing")
	}
}

func openMemorySQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite memory db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func mustExecSQL(t *testing.T, db *sql.DB, script string) {
	t.Helper()
	if _, err := db.Exec(script); err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}
