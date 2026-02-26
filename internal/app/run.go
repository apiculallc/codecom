package app

import (
	"fmt"
	"io"
	"os"
)

const Version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "codecom %s\n", Version)
		fmt.Fprintln(stderr, "tui mode is not implemented yet")
		return 1
	}

	switch args[0] {
	case "scan":
		if err := RunScan(args[1:], stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "scan error: %v\n", err)
			return 1
		}
		return 0
	case "tui":
		fmt.Fprintf(stderr, "codecom %s\n", Version)
		fmt.Fprintln(stderr, "tui mode is not implemented yet")
		return 1
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
}

func defaultCodexDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".codex"
	}
	return home + string(os.PathSeparator) + ".codex"
}
