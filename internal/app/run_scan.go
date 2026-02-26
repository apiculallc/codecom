package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"codecom/internal/output"
	"codecom/internal/sessionindex"
)

func RunScan(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)

	jsonMode := fs.Bool("json", false, "emit JSONL")
	asciiMode := fs.Bool("ascii", false, "emit ASCII table")
	codexDir := fs.String("codex-dir", defaultCodexDir(), "path to codex data root")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional args: %s", strings.Join(fs.Args(), " "))
	}
	if *jsonMode == *asciiMode {
		return errors.New("specify exactly one of --json or --ascii")
	}

	resolved, err := expandHome(*codexDir)
	if err != nil {
		return err
	}

	fmt.Fprintf(stderr, "codecom %s\n", Version)
	res, err := sessionindex.Scan(resolved)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(stderr, "warning: %s:%d: %s\n", w.SessionFile, w.Line, w.Message)
	}

	if *jsonMode {
		return output.WriteJSONL(stdout, res)
	}
	return output.WriteASCII(stdout, res)
}

func expandHome(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if path == "~" {
		home := defaultCodexDir()
		return filepath.Dir(home), nil
	}
	if strings.HasPrefix(path, "~/") {
		home := filepath.Dir(defaultCodexDir())
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}
