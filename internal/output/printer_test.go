package output_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

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

	got := stderr.String()
	expected := "jerry: starting workflow\n"
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
	printer.StepSuccess("context", 1200*time.Millisecond, 0, 0, 0, 0)

	got := stderr.String()
	expected := "  ✓ context (1.2s)\n"
	if got != expected {
		t.Errorf("StepSuccess output = %q, want %q", got, expected)
	}
}

func TestStepSuccessWithMetrics(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.StepSuccess("generate", 82*time.Second, 11, 8, 40000, 5000)

	got := stderr.String()
	expected := "  ✓ generate (1m 22s, 11 iter, 8 tools, 45k tokens)\n"
	if got != expected {
		t.Errorf("StepSuccess with metrics output = %q, want %q", got, expected)
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
	expected := "jerry: warning: step 'gate' uses unsupported feature\n"
	if got != expected {
		t.Errorf("Warning output = %q, want %q", got, expected)
	}
}

func TestWorkflowComplete(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.WorkflowComplete(4500*time.Millisecond, "run_abc123", 0)

	got := stderr.String()
	expected := "jerry: Workflow completed in 4.5s (run: run_abc123)\n"
	if got != expected {
		t.Errorf("WorkflowComplete output = %q, want %q", got, expected)
	}
}

func TestWorkflowCompleteWithTokens(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.WorkflowComplete(135*time.Second, "run_abc123", 85000)

	got := stderr.String()
	expected := "jerry: Workflow completed in 2m 15s (run: run_abc123, 85k tokens)\n"
	if got != expected {
		t.Errorf("WorkflowComplete with tokens output = %q, want %q", got, expected)
	}
}

func TestWorkflowFailed(t *testing.T) {
	printer, _, stderr := newTestPrinter()
	printer.WorkflowFailed("validate", "script exited with code 1")

	got := stderr.String()
	if !strings.Contains(got, "validate") {
		t.Errorf("WorkflowFailed should contain step name, got %q", got)
	}
	if !strings.Contains(got, "script exited with code 1") {
		t.Errorf("WorkflowFailed should contain error message, got %q", got)
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
	printer.StepSuccess("step", time.Second, 0, 0, 0, 0)
	printer.StepFailed("step", "err")
	printer.StepSkipped("step", "reason")
	printer.StepOutput("output")
	printer.Warning("warn")
	printer.WorkflowComplete(time.Second, "run_1", 0)
	printer.WorkflowFailed("step", "err")
	printer.ValidationResult("f.yaml", true, "ok")

	if stdout.Len() != 0 {
		t.Errorf("Expected all output on stderr, but stdout has: %q", stdout.String())
	}
}
