package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTUICreatesConfigAtOverridePath(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := RunTUI([]string{"--codex-dir", root}, &out, &errOut); err != nil {
		t.Fatalf("RunTUI error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "codecom.toml")); err != nil {
		t.Fatalf("expected config file at override path: %v", err)
	}
}

func TestRunTUIRendersPanesInNonInteractiveMode(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "sessions", "2026", "02", "26", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "" +
		"{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/tmp/proj\",\"session_id\":\"sid-1\"}}\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := RunTUI([]string{"--codex-dir", root}, &out, &errOut); err != nil {
		t.Fatalf("RunTUI error: %v", err)
	}
	got := out.String()
	for _, expected := range []string{"Source", "Target", "Sessions", "sid-1"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, got)
		}
	}
}

func TestRunTUIUsesProvidedTargetRoot(t *testing.T) {
	root := t.TempDir()
	targetRoot := filepath.Join(t.TempDir(), "custom-target-root")
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := RunTUI([]string{"--codex-dir", root, "--target-root", targetRoot}, &out, &errOut); err != nil {
		t.Fatalf("RunTUI error: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "custom-target-root") {
		t.Fatalf("expected output to include custom target root, got %q", got)
	}
}

func TestRunTUIFailsForMissingTargetRoot(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := RunTUI([]string{"--codex-dir", root, "--target-root", missing}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error for missing target root")
	}
	if !strings.Contains(err.Error(), "invalid target root") {
		t.Fatalf("expected invalid target root error, got %v", err)
	}
}

func TestRunTUIFailsForFileTargetRoot(t *testing.T) {
	root := t.TempDir()
	fileRoot := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := RunTUI([]string{"--codex-dir", root, "--target-root", fileRoot}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error for file target root")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not a directory error, got %v", err)
	}
}
