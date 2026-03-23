package output_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kilupskalvis/motif/internal/output"
)

func newTestPrinter() (*output.Printer, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return output.NewPrinter(stdout, stderr), stdout, stderr
}

func TestInfo(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.Info("starting %s", "pipeline")

	got := stderr.String()
	expected := "motif: starting pipeline\n"
	if got != expected {
		t.Errorf("Info output = %q, want %q", got, expected)
	}
}

func TestInfo_GoesToStderr(t *testing.T) {
	printer, stdout, _ := newTestPrinter()
	printer.Info("message")

	if stdout.Len() != 0 {
		t.Errorf("Info wrote to stdout: %q, expected nothing", stdout.String())
	}
}

func TestStepStart(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.StepStart("context")

	got := stderr.String()
	expected := "  ▸ context ...\n"
	if got != expected {
		t.Errorf("StepStart output = %q, want %q", got, expected)
	}
}

func TestStepSuccess(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.StepSuccess("context", 1200*time.Millisecond)

	got := stderr.String()
	expected := "  ✓ context (1.2s)\n"
	if got != expected {
		t.Errorf("StepSuccess output = %q, want %q", got, expected)
	}
}

func TestStepFailed(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.StepFailed("validate", "script exited with code 1")

	got := stderr.String()
	expected := "  ✗ validate — script exited with code 1\n"
	if got != expected {
		t.Errorf("StepFailed output = %q, want %q", got, expected)
	}
}

func TestStepSkipped(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.StepSkipped("generate", "agent steps require Phase 2")

	got := stderr.String()
	expected := "  ⊘ generate — agent steps require Phase 2\n"
	if got != expected {
		t.Errorf("StepSkipped output = %q, want %q", got, expected)
	}
}

func TestStepOutput(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.StepOutput("some output line")

	got := stderr.String()
	expected := "    some output line\n"
	if got != expected {
		t.Errorf("StepOutput output = %q, want %q", got, expected)
	}
}

func TestWarning(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.Warning("step '%s' uses unsupported feature", "gate")

	got := stderr.String()
	expected := "motif: warning: step 'gate' uses unsupported feature\n"
	if got != expected {
		t.Errorf("Warning output = %q, want %q", got, expected)
	}
}

func TestPipelineComplete(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.PipelineComplete(4500*time.Millisecond, "run_abc123")

	got := stderr.String()
	expected := "motif: Pipeline completed in 4.5s (run: run_abc123)\n"
	if got != expected {
		t.Errorf("PipelineComplete output = %q, want %q", got, expected)
	}
}

func TestPipelineFailed(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.PipelineFailed("validate", "script exited with code 1", "run_abc123")

	got := stderr.String()
	if !strings.Contains(got, "validate") {
		t.Errorf("PipelineFailed should contain step name, got %q", got)
	}
	if !strings.Contains(got, "run_abc123") {
		t.Errorf("PipelineFailed should contain run ID, got %q", got)
	}
}

func TestValidationResult_Valid(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.ValidationResult("feature.yaml", true, "valid (5 steps)")

	got := stderr.String()
	expected := "  ✓ feature.yaml — valid (5 steps)\n"
	if got != expected {
		t.Errorf("ValidationResult output = %q, want %q", got, expected)
	}
}

func TestValidationResult_Invalid(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.ValidationResult("broken.yaml", false, "error: missing name field")

	got := stderr.String()
	expected := "  ✗ broken.yaml — error: missing name field\n"
	if got != expected {
		t.Errorf("ValidationResult output = %q, want %q", got, expected)
	}
}

func TestAllOutputGoesToStderr(t *testing.T) {
	printer, stdout, _ := newTestPrinter()

	printer.Info("info")
	printer.StepStart("step")
	printer.StepSuccess("step", time.Second)
	printer.StepFailed("step", "err")
	printer.StepSkipped("step", "reason")
	printer.StepOutput("output")
	printer.Warning("warn")
	printer.PipelineComplete(time.Second, "run_1")
	printer.PipelineFailed("step", "err", "run_1")
	printer.ValidationResult("f.yaml", true, "ok")

	if stdout.Len() != 0 {
		t.Errorf("Expected all output on stderr, but stdout has: %q", stdout.String())
	}
}
