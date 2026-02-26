package app

import (
	"fmt"
	"io"
	"os"
)

const Version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if err := RunTUI(nil, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "tui error: %v\n", err)
			return 1
		}
		return 0
	}

	switch args[0] {
	case "scan":
		if err := RunScan(args[1:], stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "scan error: %v\n", err)
			return 1
		}
		return 0
	case "tui":
		if err := RunTUI(args[1:], stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "tui error: %v\n", err)
			return 1
		}
		return 0
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
