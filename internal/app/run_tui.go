package app

import (
	"flag"
	"fmt"
	"io"
	"os"

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

	res, err := sessionindex.Scan(resolved)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(stderr, "warning: %s:%d: %s\n", w.SessionFile, w.Line, w.Message)
	}

	m := tui.NewModel(res.Sessions)
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
