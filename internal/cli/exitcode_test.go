package cli

import (
	"errors"
	"testing"
)

func TestExecExitImplementsExitCoder(t *testing.T) {
	err := error(execExit{code: 4})
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatal("execExit must satisfy the ExitCode() interface")
	}
	if ec.ExitCode() != 4 {
		t.Errorf("ExitCode = %d, want 4", ec.ExitCode())
	}
}
