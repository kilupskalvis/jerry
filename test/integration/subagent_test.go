package integration_test

import (
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/run"
)

func TestSubagent_ParentInvokesChild(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("test-subagent", "steps:\n  - agent: parent\n").
		withAgent("test-subagent", "parent.md", `---
name: parent
model: claude-sonnet-4-6
tools:
  - scanner
---
You are a triage agent. Delegate scanning to the scanner tool.
`).
		withAgent("test-subagent", "scanner.md", `---
name: scanner
model: claude-sonnet-4-6
---
You are a security scanner. Report findings.
`).
		withLLMResponses(
			toolCallResponse("scanner", `{"task":"Check for vulnerabilities"}`),
			textResponse("No vulnerabilities found."),
			textResponse("Scan complete. No issues found."),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want completed", result.runState.Status)
	}
}

func TestSubagent_ResultFlowsBackToParent(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("test-flow", "steps:\n  - agent: coordinator\n").
		withAgent("test-flow", "coordinator.md", `---
name: coordinator
model: claude-sonnet-4-6
tools:
  - helper
---
Coordinate work. Use helper for subtasks.
`).
		withAgent("test-flow", "helper.md", `---
name: helper
model: claude-sonnet-4-6
---
Help with tasks.
`).
		withLLMResponses(
			toolCallResponse("helper", `{"task":"Count files"}`),
			textResponse("Found 42 files."),
			textResponse("Helper reported 42 files."),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want completed", result.runState.Status)
	}

	// Verify the subagent result was sent back to the parent
	if len(result.llmCalls) < 3 {
		t.Fatal("expected at least 3 LLM calls (parent + subagent + parent)")
	}
	thirdCall := result.llmCalls[2]
	found := false
	for _, msg := range thirdCall.Messages {
		if strings.Contains(msg.Content, "42 files") {
			found = true
			break
		}
	}
	if !found {
		t.Error("subagent result should flow back to parent as tool result")
	}
}
