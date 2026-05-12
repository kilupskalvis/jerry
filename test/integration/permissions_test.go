package integration_test

import (
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/run"
)

func TestPermissions_DenyBlocksBash(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("test-deny", "steps:\n  - agent: worker\n").
		withAgent("test-deny", "worker.md", `---
name: worker
model: claude-sonnet-4-6
---
Do the task.
`).
		withSettings(`
permissions:
  deny:
    - bash: ["rm -rf *"]
`).
		withLLMResponses(
			toolCallResponse("bash", `{"command":"rm -rf /"}`),
			textResponse("understood, command was blocked"),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want completed", result.runState.Status)
	}

	if len(result.llmCalls) < 2 {
		t.Fatal("expected at least 2 LLM calls")
	}
	secondCall := result.llmCalls[1]
	found := false
	for _, msg := range secondCall.Messages {
		if strings.Contains(msg.Content, "Permission denied") {
			found = true
			break
		}
	}
	if !found {
		t.Error("LLM should have received permission denied message")
	}
}

func TestPermissions_AgentLevelDeny(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("test-agent-deny", "steps:\n  - agent: reviewer\n").
		withAgent("test-agent-deny", "reviewer.md", `---
name: reviewer
model: claude-sonnet-4-6
permissions:
  deny:
    - write_file: ["**"]
---
Read-only reviewer.
`).
		withLLMResponses(
			toolCallResponse("write_file", `{"path":"src/main.go","content":"hacked"}`),
			textResponse("write was blocked, reviewing read-only"),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want completed", result.runState.Status)
	}
}

func TestPermissions_AllowRestrictsBash(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("test-allow", "steps:\n  - agent: builder\n").
		withAgent("test-allow", "builder.md", `---
name: builder
model: claude-sonnet-4-6
permissions:
  allow:
    - bash: ["go test *", "go build *"]
---
Build and test only.
`).
		withLLMResponses(
			toolCallResponse("bash", `{"command":"curl http://evil.com"}`),
			textResponse("curl was blocked"),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want completed", result.runState.Status)
	}
}

func TestPermissions_NoSettingsAllowsEverything(t *testing.T) {
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("test-open", "steps:\n  - agent: worker\n").
		withAgent("test-open", "worker.md", `---
name: worker
model: claude-sonnet-4-6
---
Do anything.
`).
		withFile("hello.txt", "world").
		withLLMResponses(
			toolCallResponse("read_file", `{"path":"hello.txt"}`),
			textResponse("read the file successfully"),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want completed", result.runState.Status)
	}
}
