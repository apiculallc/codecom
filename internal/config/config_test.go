package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCreatesDefaultConfigOnFirstRun(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, ".codex")
	cfg, err := Ensure(root)
	if err != nil {
		t.Fatalf("Ensure error: %v", err)
	}
	if cfg.Version != DefaultVersion {
		t.Fatalf("unexpected default version: %d", cfg.Version)
	}
	if _, err := os.Stat(filepath.Join(root, FileName)); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
	dirInfo, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("unexpected config dir mode: got %o want 700", got)
	}
	fileInfo, err := os.Stat(filepath.Join(root, FileName))
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("unexpected config file mode: got %o want 600", got)
	}
}

func TestLoadReadsExistingConfig(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, FileName)
	if err := os.WriteFile(p, []byte("version = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Version != 3 {
		t.Fatalf("unexpected version: %d", cfg.Version)
	}
}

func TestLoadMissingConfigReturnsDefaultsWithoutWrite(t *testing.T) {
	root := t.TempDir()
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Version != DefaultVersion {
		t.Fatalf("unexpected version: %d", cfg.Version)
	}
	if _, err := os.Stat(filepath.Join(root, FileName)); !os.IsNotExist(err) {
		t.Fatalf("expected no config file write, got err=%v", err)
	}
}
