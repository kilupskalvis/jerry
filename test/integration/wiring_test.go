package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

func TestTriggerReachesAgent(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("review", "steps:\n  - agent: reviewer\n").
		withAgent("review", "reviewer.md", `---
name: reviewer
model: claude-sonnet-4-6
---
Review the pull request for issues.
`).
		withTrigger(trigger.TriggerData{
			Type:   "pull_request",
			Source: "github",
			Intent: "Fix null pointer in auth",
			URL:    "https://github.com/org/repo/pull/99",
			Author: "alice",
		}).
		withLLMResponses(textResponse("No issues found.")).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Fatalf("status = %q, want %q", result.runState.Status, run.StatusCompleted)
	}
	if len(result.llmCalls) != 1 {
		t.Fatalf("got %d LLM calls, want 1", len(result.llmCalls))
	}

	prompt := result.llmCalls[0].SystemPrompt
	for _, want := range []string{
		"## Trigger",
		"Type: pull_request",
		"Source: github",
		"Intent: Fix null pointer in auth",
		"URL: https://github.com/org/repo/pull/99",
		"Author: alice",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestMinimalTriggerReachesAgent(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("review", "steps:\n  - agent: reviewer\n").
		withAgent("review", "reviewer.md", `---
name: reviewer
model: claude-sonnet-4-6
---
Review the code.
`).
		withTrigger(trigger.TriggerData{
			Type:   "manual",
			Source: "cli",
		}).
		withLLMResponses(textResponse("Done.")).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}

	prompt := result.llmCalls[0].SystemPrompt
	if !strings.Contains(prompt, "Type: manual") {
		t.Error("system prompt missing 'Type: manual'")
	}
	if !strings.Contains(prompt, "Source: cli") {
		t.Error("system prompt missing 'Source: cli'")
	}
	for _, absent := range []string{"Intent:", "URL:", "Author:"} {
		if strings.Contains(prompt, absent) {
			t.Errorf("system prompt should not contain empty field %q", absent)
		}
	}
}

func TestContextFlowsBetweenAgents(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("multi", "steps:\n  - agent: planner\n  - agent: reviewer\n").
		withAgent("multi", "planner.md", `---
name: planner
model: claude-sonnet-4-6
---
Create a plan for the task.
`).
		withAgent("multi", "reviewer.md", `---
name: reviewer
model: claude-sonnet-4-6
---
Use the previous plan to make a decision.
`).
		withTrigger(trigger.TriggerData{
			Type:   "ticket",
			Source: "github",
			Intent: "Fix auth issues",
			Author: "alice",
		}).
		withLLMResponses(
			textResponse("Plan: fix token expiry check in middleware."),
			textResponse("Decision: plan looks correct, proceed."),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if len(result.llmCalls) < 2 {
		t.Fatalf("got %d LLM calls, want at least 2", len(result.llmCalls))
	}

	secondPrompt := result.llmCalls[1].SystemPrompt
	for _, want := range []string{
		"## Trigger",
		"Intent: Fix auth issues",
		"## Previous Steps",
		"### planner",
		"fix token expiry check in middleware",
		"Use the previous plan",
	} {
		if !strings.Contains(secondPrompt, want) {
			t.Errorf("second agent system prompt missing %q", want)
		}
	}
}

func TestAgentToolCallLoop(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("audit", "steps:\n  - agent: auditor\n").
		withAgent("audit", "auditor.md", `---
name: auditor
model: claude-sonnet-4-6
---
Audit the codebase.
`).
		withFile("src/app.go", "package src\nfunc Run() {}\n").
		withFile("src/util.go", "package src\nfunc Help() {}\n").
		withLLMResponses(
			toolCallResponse("bash", `{"command": "find src -name '*.go'"}`),
			toolCallResponse("read_file", `{"path": "src/app.go"}`),
			textResponse("Audit complete. Found Run function in app.go."),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want %q", result.runState.Status, run.StatusCompleted)
	}
	if len(result.llmCalls) != 3 {
		t.Fatalf("got %d LLM calls, want 3", len(result.llmCalls))
	}

	for i := 1; i < len(result.llmCalls); i++ {
		prev := len(result.llmCalls[i-1].Messages)
		curr := len(result.llmCalls[i].Messages)
		if curr <= prev {
			t.Errorf("LLM call %d has %d messages, expected more than call %d (%d messages)",
				i, curr, i-1, prev)
		}
	}
}

func TestAgentThenScript(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("pipeline", "steps:\n  - agent: planner\n  - name: check\n    run: cat \"$JERRY_CONTEXT_FILE\"\n").
		withAgent("pipeline", "planner.md", `---
name: planner
model: claude-sonnet-4-6
---
Create a plan.
`).
		withTrigger(trigger.TriggerData{
			Type:   "pull_request",
			Source: "github",
			Intent: "Update docs",
		}).
		withLLMResponses(textResponse("Plan: update README with new API docs.")).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Fatalf("status = %q, want %q", result.runState.Status, run.StatusCompleted)
	}
	if len(result.runState.StepResults) != 2 {
		t.Fatalf("got %d step results, want 2", len(result.runState.StepResults))
	}

	scriptOutput := result.runState.StepResults[1].Stdout
	if !strings.Contains(scriptOutput, `"type": "pull_request"`) {
		t.Errorf("script output missing trigger type, got: %s", scriptOutput)
	}
	if !strings.Contains(scriptOutput, "update README with new API docs") {
		t.Errorf("script output missing agent output, got: %s", scriptOutput)
	}
}

func TestCustomToolExecution(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	toolsDir := filepath.Join(env.jerryDir, "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "greet.yaml"), []byte(`
description: Greet someone
parameters:
  name:
    type: string
    required: true
run: echo "Hello, $TOOL_NAME!"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	result := env.
		withWorkflow("custom", "steps:\n  - agent: greeter\n").
		withAgent("custom", "greeter.md", `---
name: greeter
model: claude-sonnet-4-6
tools:
  - greet
---
Greet the user by name.
`).
		withLLMResponses(
			toolCallResponse("greet", `{"name": "World"}`),
			textResponse("Greeted successfully."),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want %q", result.runState.Status, run.StatusCompleted)
	}

	foundGreeting := false
	if len(result.llmCalls) >= 2 {
		for _, msg := range result.llmCalls[1].Messages {
			if msg.Role == llm.RoleTool && strings.Contains(msg.Content, "Hello, World!") {
				foundGreeting = true
				break
			}
		}
	}
	if !foundGreeting {
		t.Error("expected tool result containing 'Hello, World!'")
	}
}

func TestScriptFailureStopsWorkflow(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("failing", "steps:\n  - name: fail-step\n    run: exit 1\n  - agent: never-runs\n").
		withAgent("failing", "never-runs.md", `---
name: never-runs
model: claude-sonnet-4-6
---
This should not execute.
`).
		run()

	if result.err == nil {
		t.Fatal("expected error from failing script")
	}
	if result.runState.Status != run.StatusFailed {
		t.Errorf("status = %q, want %q", result.runState.Status, run.StatusFailed)
	}
	if len(result.llmCalls) != 0 {
		t.Errorf("agent should not have been called, got %d LLM calls", len(result.llmCalls))
	}
}

func TestStatePersisted(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t).
		withWorkflow("stateful", "steps:\n  - agent: worker\n  - name: verify\n    run: echo \"verified\"\n").
		withAgent("stateful", "worker.md", `---
name: worker
model: claude-sonnet-4-6
---
Do the work.
`).
		withLLMResponses(textResponse("Work complete."))

	result := env.run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}

	runsDir := filepath.Join(env.jerryDir, "runs")
	stateStore := run.NewFileStateStore(runsDir)
	loaded, loadErr := stateStore.LoadRun(result.runState.RunID)
	if loadErr != nil {
		t.Fatalf("load persisted state: %v", loadErr)
	}

	if loaded.Status != run.StatusCompleted {
		t.Errorf("persisted status = %q, want %q", loaded.Status, run.StatusCompleted)
	}
	if len(loaded.StepResults) != 2 {
		t.Fatalf("persisted step results = %d, want 2", len(loaded.StepResults))
	}
	if loaded.StepResults[0].Name != "worker" {
		t.Errorf("step 0 name = %q, want %q", loaded.StepResults[0].Name, "worker")
	}
	if loaded.StepResults[1].Name != "verify" {
		t.Errorf("step 1 name = %q, want %q", loaded.StepResults[1].Name, "verify")
	}
}
