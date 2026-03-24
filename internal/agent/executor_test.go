package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/motif/internal/agent"
	"github.com/kilupskalvis/motif/internal/contextstore"
	"github.com/kilupskalvis/motif/internal/llm"
	"github.com/kilupskalvis/motif/internal/output"
	"github.com/kilupskalvis/motif/internal/pipeline"
	"github.com/kilupskalvis/motif/internal/tools"
)

// mockLLMClient returns predefined responses in sequence.
type mockLLMClient struct {
	responses []*llm.Response
	callIndex int
}

func (m *mockLLMClient) Send(_ context.Context, _ string, _ []llm.Message, _ []llm.ToolDef) (*llm.Response, error) {
	if m.callIndex >= len(m.responses) {
		return &llm.Response{
			Content:    "no more responses",
			StopReason: "end_turn",
			Usage:      llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
		}, nil
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp, nil
}

func setupTestAgent(t *testing.T, dir string) string {
	t.Helper()
	agentPath := filepath.Join(dir, "test-agent.md")
	content := `---
name: test-agent
model: claude-sonnet-4-6
context_access:
  - trigger
output_key: result
output_schema:
  summary: string
---

# Test Agent

You are a test agent. Return a JSON summary.
`
	if err := os.WriteFile(agentPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return agentPath
}

func TestAgentExecutor_CanExecute_True(t *testing.T) {
	exec := agent.NewExecutor(nil, nil, nil, nil)
	step := pipeline.Step{Name: "gen", Agent: "./agents/generate.md"}
	if !exec.CanExecute(step) {
		t.Error("should return true for steps with Agent set")
	}
}

func TestAgentExecutor_CanExecute_False(t *testing.T) {
	exec := agent.NewExecutor(nil, nil, nil, nil)
	step := pipeline.Step{Name: "test", Script: "echo hi"}
	if exec.CanExecute(step) {
		t.Error("should return false for steps with Script set")
	}
}

func TestAgentExecutor_CanExecute_Empty(t *testing.T) {
	exec := agent.NewExecutor(nil, nil, nil, nil)
	step := pipeline.Step{Name: "empty"}
	if exec.CanExecute(step) {
		t.Error("should return false for empty steps")
	}
}

func TestAgentExecutor_Execute_NilClient(t *testing.T) {
	exec := agent.NewExecutor(nil, nil, nil, nil)
	step := pipeline.Step{Name: "gen", Agent: "./agents/test.md"}

	_, err := exec.Execute(context.Background(), step, nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestAgentExecutor_Execute_Success(t *testing.T) {
	dir := t.TempDir()
	agentPath := setupTestAgent(t, dir)

	reg := tools.NewRegistry(dir, nil)
	loader := agent.NewLoader(reg.KnownToolNames(), "")
	printer := output.NewPrinter(os.Stdout, os.Stderr)

	mockClient := &mockLLMClient{
		responses: []*llm.Response{
			{
				Content:    `{"summary": "test completed"}`,
				StopReason: "end_turn",
				Usage:      llm.TokenUsage{InputTokens: 100, OutputTokens: 50},
			},
		},
	}

	exec := agent.NewExecutor(loader, reg, mockClient, printer)
	store := contextstore.NewStore("test-run", contextstore.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: "test",
	})

	step := pipeline.Step{Name: "gen", Agent: agentPath}
	result, err := exec.Execute(context.Background(), step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OutputKeyOverride != "result" {
		t.Errorf("expected OutputKeyOverride 'result', got %q", result.OutputKeyOverride)
	}

	if result.Data == nil {
		t.Fatal("expected parsed data, got nil")
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result.Data)
	}
	if data["summary"] != "test completed" {
		t.Errorf("expected summary 'test completed', got %v", data["summary"])
	}
}

func TestAgentExecutor_Execute_TriggerAlwaysPresent(t *testing.T) {
	dir := t.TempDir()

	// Agent that only has context_access: ["codebase"] — does NOT list trigger.
	agentPath := filepath.Join(dir, "no-trigger-access.md")
	content := `---
name: no-trigger-agent
model: claude-sonnet-4-6
context_access:
  - codebase
output_key: result
output_schema:
  summary: string
---

# Agent

Return a summary.
`
	os.WriteFile(agentPath, []byte(content), 0o644)

	reg := tools.NewRegistry(dir, nil)
	loader := agent.NewLoader(reg.KnownToolNames(), "")
	printer := output.NewPrinter(os.Stdout, os.Stderr)

	// The mock client verifies it receives the trigger in the system message.
	var capturedSystem string
	mockClient := &capturingMockClient{
		response: &llm.Response{
			Content:    `{"summary": "done"}`,
			StopReason: "end_turn",
			Usage:      llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
		capturedSystem: &capturedSystem,
	}

	exec := agent.NewExecutor(loader, reg, mockClient, printer)
	store := contextstore.NewStore("test-run", contextstore.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: "build something",
	})

	step := pipeline.Step{Name: "gen", Agent: agentPath}
	_, err := exec.Execute(context.Background(), step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The system message should contain the trigger data even though
	// context_access only lists "codebase".
	if capturedSystem == "" {
		t.Fatal("system message not captured")
	}
	if !contains(capturedSystem, "build something") {
		t.Errorf("system message should contain trigger intent, got %q", capturedSystem[:min(200, len(capturedSystem))])
	}
}

type capturingMockClient struct {
	response       *llm.Response
	capturedSystem *string
}

func (m *capturingMockClient) Send(_ context.Context, system string, _ []llm.Message, _ []llm.ToolDef) (*llm.Response, error) {
	*m.capturedSystem = system
	return m.response, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
