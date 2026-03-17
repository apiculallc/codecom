package app

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunScanJSONWritesDataToStdoutAndWarningsToStderr(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "sessions", "2026", "02", "26", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "" +
		"{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/proj\",\"session_id\":\"sid-1\"}}\n" +
		"bad-json\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := RunScan([]string{"--json", "--codex-dir", root}, &out, &errOut)
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	if !strings.Contains(out.String(), "\"session_id\":\"sid-1\"") {
		t.Fatalf("expected json output on stdout, got %q", out.String())
	}
	errText := errOut.String()
	if !strings.Contains(errText, "codecom ") {
		t.Fatalf("expected version header on stderr, got %q", errText)
	}
	if !strings.Contains(errText, "warning:") {
		t.Fatalf("expected warning details on stderr, got %q", errText)
	}
}

func TestRunScanASCIIMode(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "sessions", "2026", "02", "26", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/proj\",\"session_id\":\"sid-2\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := RunScan([]string{"--ascii", "--codex-dir", root}, &out, &errOut)
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if !strings.Contains(out.String(), "SESSION_ID") || !strings.Contains(out.String(), "sid-2") {
		t.Fatalf("expected ascii output on stdout, got %q", out.String())
	}
}

func TestRunScanRequiresExactlyOneMode(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	err := RunScan([]string{}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error for missing mode")
	}
	err = RunScan([]string{"--json", "--ascii"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error for two modes")
	}
}

func TestRunScanDoesNotCreateConfigFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "sessions", "2026", "02", "26", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/proj\",\"session_id\":\"sid-2\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := RunScan([]string{"--json", "--codex-dir", root}, &out, &errOut); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "codecom.toml")); !os.IsNotExist(err) {
		t.Fatalf("scan should not write config file, got stat err=%v", err)
	}
}

func TestRunScanReadOnlyDoesNotMutateJSONLOrSQLite(t *testing.T) {
	root := t.TempDir()
	sessionFile := filepath.Join(root, "sessions", "2026", "02", "26", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionFile), 0o755); err != nil {
		t.Fatal(err)
	}
	initialJSONL := "{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/proj\",\"session_id\":\"sid-1\"}}\n"
	if err := os.WriteFile(sessionFile, []byte(initialJSONL), 0o644); err != nil {
		t.Fatal(err)
	}

	sqlitePath := filepath.Join(root, "state_5.sqlite")
	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE threads (id TEXT PRIMARY KEY, cwd TEXT, rollout_path TEXT);
INSERT INTO threads(id, cwd, rollout_path) VALUES('sid-1', '/tmp/proj', '/tmp/a.jsonl');
`); err != nil {
		_ = db.Close()
		t.Fatalf("seed sqlite: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	beforeJSON, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	beforeDB, err := os.ReadFile(sqlitePath)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := RunScan([]string{"--json", "--codex-dir", root}, &out, &errOut); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	afterJSON, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	afterDB, err := os.ReadFile(sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterJSON) != string(beforeJSON) {
		t.Fatalf("scan must not rewrite jsonl\nbefore=%q\nafter=%q", string(beforeJSON), string(afterJSON))
	}
	if string(afterDB) != string(beforeDB) {
		t.Fatal("scan must not mutate sqlite database file")
	}
}
