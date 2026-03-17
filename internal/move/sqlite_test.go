package move

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRewritePlanSQLiteUpdatesOnlySelectedIDs(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "state_5.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	mustExecSQL(t, db, `
CREATE TABLE threads (
  id TEXT PRIMARY KEY,
  cwd TEXT,
  rollout_path TEXT
);
INSERT INTO threads (id, cwd, rollout_path) VALUES
  ('sid-1', '/old/repo/a', '/old/.codex/sessions/a.jsonl'),
  ('sid-2', '/old/repo/b', '/old/.codex/sessions/b.jsonl');
`)

	plan := Plan{
		SourceRoot: "/old",
		TargetRoot: "/new",
		Items: []PlanItem{
			{SessionID: "sid-1"},
			{SessionID: "sid-1"}, // duplicate should be deduped
		},
	}
	if err := RewritePlanSQLite(root, plan); err != nil {
		t.Fatalf("RewritePlanSQLite error: %v", err)
	}

	assertThreadCWD(t, db, "sid-1", "/new/repo/a")
	assertThreadCWD(t, db, "sid-2", "/old/repo/b")
}

func TestRewritePlanSQLiteFailsPreflightWithoutThreads(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "state_5.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	// Intentionally no threads table.
	if err := RewritePlanSQLite(root, Plan{SourceRoot: "/old", TargetRoot: "/new"}); err == nil {
		t.Fatal("expected sqlite preflight error")
	}
}

func TestPlanSessionIDsDeterministic(t *testing.T) {
	ids := planSessionIDs(Plan{
		Items: []PlanItem{
			{SessionID: "sid-b"},
			{SessionID: ""},
			{SessionID: "sid-a"},
			{SessionID: "sid-b"},
		},
	})
	if len(ids) != 2 || ids[0] != "sid-a" || ids[1] != "sid-b" {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}

func mustExecSQL(t *testing.T, db *sql.DB, script string) {
	t.Helper()
	if _, err := db.Exec(script); err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}

func assertThreadCWD(t *testing.T, db *sql.DB, id, want string) {
	t.Helper()
	var got string
	if err := db.QueryRow("SELECT cwd FROM threads WHERE id = ?", id).Scan(&got); err != nil {
		t.Fatalf("query cwd for %s: %v", id, err)
	}
	if got != want {
		t.Fatalf("cwd mismatch for %s: got %q want %q", id, got, want)
	}
}
