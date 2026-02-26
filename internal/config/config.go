package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	FileName       = "codecom.toml"
	DefaultVersion = 1
)

type Config struct {
	Version int
}

func Default() Config {
	return Config{Version: DefaultVersion}
}

func Path(codexDir string) string {
	return filepath.Join(codexDir, FileName)
}

func Load(codexDir string) (Config, error) {
	cfgPath := Path(codexDir)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}
	return parse(data)
}

func Ensure(codexDir string) (Config, error) {
	cfgPath := Path(codexDir)
	if _, err := os.Stat(cfgPath); err == nil {
		return Load(codexDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := os.WriteFile(cfgPath, []byte(serialize(cfg)), 0o644); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parse(data []byte) (Config, error) {
	cfg := Default()
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return Config{}, fmt.Errorf("invalid config line: %q", line)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "version":
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return Config{}, fmt.Errorf("invalid version: %q", val)
			}
			cfg.Version = n
		}
	}
	return cfg, nil
}

func serialize(cfg Config) string {
	return "# codecom configuration\n" +
		"version = " + strconv.Itoa(cfg.Version) + "\n"
}
