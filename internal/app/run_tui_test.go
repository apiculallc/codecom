package app

import (
	"bytes"
	"os"
	"path/filepath"
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
