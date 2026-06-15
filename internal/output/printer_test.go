package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/output"
)

func newTestPrinter() (*output.Printer, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return output.NewPrinter(stdout, stderr), stdout, stderr
}

func TestInfo(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.Info("starting %s", "workflow")
	if !strings.Contains(stderr.String(), "starting workflow") {
		t.Errorf("Info output = %q", stderr.String())
	}
}

func TestWarning(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.Warning("could not load %s", ".env")
	out := stderr.String()
	if !strings.Contains(out, "warning") || !strings.Contains(out, ".env") {
		t.Errorf("Warning output = %q", out)
	}
}

func TestValidationResult(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.ValidationResult("review", true, "valid")
	printer.ValidationResult("feature", false, "bad step")
	out := stderr.String()
	if !strings.Contains(out, "✓ review — valid") {
		t.Errorf("valid result missing: %q", out)
	}
	if !strings.Contains(out, "✗ feature — bad step") {
		t.Errorf("invalid result missing: %q", out)
	}
}

func TestQuietSuppressesInfoAndWarning(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.SetVerbosity(output.VerbosityQuiet)
	printer.Info("hidden")
	printer.Warning("also hidden")
	if stderr.Len() != 0 {
		t.Errorf("quiet mode should suppress info/warning, got %q", stderr.String())
	}
}

func TestStdoutStaysClean(t *testing.T) {
	printer, stdout, _ := newTestPrinter()
	printer.Info("x")
	printer.Warning("y")
	printer.ValidationResult("z", true, "ok")
	if stdout.Len() != 0 {
		t.Errorf("stdout must stay clean for piping, got %q", stdout.String())
	}
}
