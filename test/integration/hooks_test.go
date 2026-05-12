package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/run"
)

func TestHooks_WorkflowComplete(t *testing.T) {
	t.Parallel()
	outFile := filepath.Join(t.TempDir(), "hook-out.txt")

	result := newTestEnv(t).
		withWorkflow("test-hooks", `
steps:
  - agent: worker
hooks:
  on_workflow_complete:
    - run: echo "workflow done" > `+outFile+`
`).
		withAgent("test-hooks", "worker.md", `---
name: worker
model: claude-sonnet-4-6
---
Do the task.
`).
		withLLMResponses(
			textResponse("done"),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want completed", result.runState.Status)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("hook did not write file: %v", err)
	}
	if !strings.Contains(string(data), "workflow done") {
		t.Errorf("got %q, want 'workflow done'", string(data))
	}
}

func TestHooks_StepComplete(t *testing.T) {
	t.Parallel()
	outFile := filepath.Join(t.TempDir(), "step-out.txt")

	result := newTestEnv(t).
		withWorkflow("test-step-hooks", `
steps:
  - agent: worker
hooks:
  on_step_complete:
    - run: echo "$JERRY_HOOK_STEP_NAME" >> `+outFile+`
`).
		withAgent("test-step-hooks", "worker.md", `---
name: worker
model: claude-sonnet-4-6
---
Do the task.
`).
		withLLMResponses(
			textResponse("done"),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("step hook did not write file: %v", err)
	}
	if !strings.Contains(string(data), "worker") {
		t.Errorf("got %q, want step name 'worker'", string(data))
	}
}

func TestHooks_NoHooksDoesNotFail(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("test-no-hooks", "steps:\n  - agent: worker\n").
		withAgent("test-no-hooks", "worker.md", `---
name: worker
model: claude-sonnet-4-6
---
Do the task.
`).
		withLLMResponses(
			textResponse("done"),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want completed", result.runState.Status)
	}
}
