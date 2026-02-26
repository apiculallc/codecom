package app

import (
	"flag"
	"fmt"
	"io"

	"codecom/internal/config"
)

func RunTUI(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	codexDir := fs.String("codex-dir", defaultCodexDir(), "path to codex data root")

	if err := fs.Parse(args); err != nil {
		return err
	}
	resolved, err := expandHome(*codexDir)
	if err != nil {
		return err
	}
	if _, err := config.Ensure(resolved); err != nil {
		return err
	}

	fmt.Fprintf(stderr, "codecom %s\n", Version)
	fmt.Fprintln(stderr, "tui mode is not implemented yet")
	return nil
}
