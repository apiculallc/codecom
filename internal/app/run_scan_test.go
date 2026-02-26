package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
