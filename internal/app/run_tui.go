package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"codecom/internal/config"
	"codecom/internal/sessionindex"
	"codecom/internal/tui"
)

func RunTUI(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	codexDir := fs.String("codex-dir", defaultCodexDir(), "path to codex data root")
	targetRoot := fs.String("target-root", defaultTargetRoot(), "path to target pane root directory")

	if err := fs.Parse(args); err != nil {
		return err
	}
	resolved, err := expandHome(*codexDir)
	if err != nil {
		return err
	}
	resolvedTargetRoot, err := expandHome(*targetRoot)
	if err != nil {
		return err
	}
	resolvedTargetRoot = filepath.Clean(resolvedTargetRoot)
	if err := validateTargetRoot(resolvedTargetRoot); err != nil {
		return err
	}
	if _, err := config.Ensure(resolved); err != nil {
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

	m := tui.NewModelWithTargetRoot(res.Sessions, resolvedTargetRoot).WithCodexRoot(resolved)
	if !isTerminalWriter(stdout) {
		_, err := io.WriteString(stdout, m.View())
		return err
	}

	p := tea.NewProgram(m, tea.WithInput(os.Stdin), tea.WithOutput(stdout))
	_, err = p.Run()
	return err
}

func isTerminalWriter(w io.Writer) bool {
	type fdWriter interface{ Fd() uintptr }
	fw, ok := w.(fdWriter)
	if !ok {
		return false
	}
	return isatty.IsTerminal(fw.Fd()) || isatty.IsCygwinTerminal(fw.Fd())
}

func defaultTargetRoot() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return home
	}
	return string(os.PathSeparator)
}

func validateTargetRoot(path string) error {
	if path == "" {
		return errors.New("target root is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("invalid target root %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("invalid target root %q: not a directory", path)
	}
	return nil
}
