package catalog

import (
	"errors"
	"testing"
)

func TestRunKeepsBothStreamsOfAFailedCommand(t *testing.T) {
	out, err := Run(nil, "sh", "-c", "echo out; echo problem >&2; exit 3")
	var run *RunError
	if !errors.As(err, &run) {
		t.Fatalf("error %v is not a RunError", err)
	}
	if out != "out\n" || run.Stderr != "problem" {
		t.Errorf("stdout %q and stderr %q, want out and problem", out, run.Stderr)
	}
}
