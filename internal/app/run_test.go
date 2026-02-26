package app

import (
	"bytes"
	"testing"
)

func TestRunUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run([]string{"unknown"}, &out, &errOut)
	if code == 0 {
		t.Fatal("expected non-zero for unknown command")
	}
}
