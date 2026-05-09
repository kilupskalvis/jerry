package workflow_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/agent"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/tool"
	"github.com/kilupskalvis/jerry/internal/workflow"
)

type mockProvider struct {
	responses []*llm.CompleteResponse
	callIndex int
}

func (m *mockProvider) Complete(_ context.Context, _ llm.CompleteParams) (*llm.CompleteResponse, error) {
	if m.callIndex >= len(m.responses) {
		return &llm.CompleteResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Content: "no more responses"},
			StopReason: llm.StopReasonEndTurn,
			Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
		}, nil
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp, nil
}

func newTestAgentExecutor(loader *agent.Loader, reg *tool.Registry, mock llm.Provider) *workflow.AgentExecutor {
	exec := workflow.NewAgentExecutor(loader, reg, "", "")
	exec.ProviderOverride = mock
	return exec
}

func setupTestAgent(t *testing.T, dir string) string {
	t.Helper()
	agentPath := filepath.Join(dir, "test-agent.md")
	content := `---
name: test-agent
model: claude-sonnet-4-6
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
	exec := workflow.NewAgentExecutor(nil, nil, "", "")
	step := workflow.Step{Name: "gen", Agent: "./agents/generate.md"}
	if !exec.CanExecute(step) {
		t.Error("should return true for steps with Agent set")
	}
}

func TestAgentExecutor_CanExecute_False(t *testing.T) {
	exec := workflow.NewAgentExecutor(nil, nil, "", "")
	step := workflow.Step{Name: "test", Run: "echo hi"}
	if exec.CanExecute(step) {
		t.Error("should return false for steps with Script set")
	}
}

func TestAgentExecutor_CanExecute_Empty(t *testing.T) {
	exec := workflow.NewAgentExecutor(nil, nil, "", "")
	step := workflow.Step{Name: "empty"}
	if exec.CanExecute(step) {
		t.Error("should return false for empty steps")
	}
}

func TestAgentExecutor_Execute_NilClient(t *testing.T) {
	exec := workflow.NewAgentExecutor(nil, nil, "", "")
	step := workflow.Step{Name: "gen", Agent: "./agents/test.md"}

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

	reg := tool.NewRegistry(dir, nil)
	loader := agent.NewLoader(reg.KnownToolNames(), "")

	mock := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: `{"summary": "test completed"}`},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
	}

	exec := newTestAgentExecutor(loader, reg, mock)

	step := workflow.Step{Name: "gen", Agent: agentPath}
	result, err := exec.Execute(context.Background(), step, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Data == "" {
		t.Fatal("expected non-empty data")
	}
	if !strings.Contains(result.Data, "test completed") {
		t.Errorf("expected data to contain 'test completed', got %q", result.Data)
	}
}
