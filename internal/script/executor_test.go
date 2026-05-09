package script_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/script"
	"github.com/kilupskalvis/jerry/internal/workflow"
)

func newTestExecutor(t *testing.T) *script.Executor {
	t.Helper()
	repoRoot := t.TempDir()
	env := map[string]string{
		"JERRY_SECRET_TEST": "secret_value",
	}
	return script.NewExecutor(repoRoot, env)
}

func TestCanExecute(t *testing.T) {
	exec := newTestExecutor(t)

	if !exec.CanExecute(workflow.Step{Run: "echo hi"}) {
		t.Error("should handle steps with Run set")
	}
	if exec.CanExecute(workflow.Step{Agent: "./agent.md"}) {
		t.Error("should not handle steps with Agent set")
	}
	if exec.CanExecute(workflow.Step{}) {
		t.Error("should not handle empty steps")
	}
}

func TestExecute_Success(t *testing.T) {
	exec := newTestExecutor(t)
	step := workflow.Step{Name: "test", Run: "echo hello"}

	output, err := exec.Execute(context.Background(), step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output.Data, "hello") {
		t.Errorf("expected output to contain 'hello', got %q", output.Data)
	}
}

func TestExecute_Failure(t *testing.T) {
	exec := newTestExecutor(t)
	step := workflow.Step{Name: "test", Run: "exit 1"}

	_, err := exec.Execute(context.Background(), step, nil)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "SCRIPT_FAILED") {
		t.Errorf("error should contain SCRIPT_FAILED, got %q", err.Error())
	}
}

func TestExecute_Timeout(t *testing.T) {
	exec := newTestExecutor(t)
	step := workflow.Step{Name: "test", Run: "sleep 60"}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := exec.Execute(ctx, step, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "SCRIPT_TIMEOUT") {
		t.Errorf("error should contain SCRIPT_TIMEOUT, got %q", err.Error())
	}
}

func TestExecute_StepName(t *testing.T) {
	exec := newTestExecutor(t)
	step := workflow.Step{Name: "my-step", Run: "echo hi"}

	output, err := exec.Execute(context.Background(), step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.StepName != "my-step" {
		t.Errorf("StepName = %q, want %q", output.StepName, "my-step")
	}
}
