package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCreatesDefaultConfigOnFirstRun(t *testing.T) {
	root := t.TempDir()
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
