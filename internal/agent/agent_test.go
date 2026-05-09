package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/kilupskalvis/jerry/internal/agent"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/tool"
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

func TestAgent_DirectResponse(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: `{"result": "done"}`},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
	}

	a := agent.NewAgent(provider, agent.WithMaxTurns(10))

	output, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != `{"result": "done"}` {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestAgent_OneToolCall(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
					},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: `{"result": "done"}`},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 200, OutputTokens: 30},
			},
		},
	}

	var dispatchedCalls []string
	testTool := tool.NewToolFunc("read_file", "Read a file", json.RawMessage(`{}`),
		func(_ context.Context, _ json.RawMessage) (string, error) {
			dispatchedCalls = append(dispatchedCalls, "read_file")
			return "file contents", nil
		},
	)

	a := agent.NewAgent(provider, agent.WithTools(testTool), agent.WithMaxTurns(10))

	output, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != `{"result": "done"}` {
		t.Errorf("unexpected output: %q", output)
	}
	if len(dispatchedCalls) != 1 || dispatchedCalls[0] != "read_file" {
		t.Errorf("expected dispatched [read_file], got %v", dispatchedCalls)
	}
}

func TestAgent_MultipleIterations(t *testing.T) {
	responses := make([]*llm.CompleteResponse, 6)
	for i := 0; i < 5; i++ {
		responses[i] = &llm.CompleteResponse{
			Message: llm.Message{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call", Name: "glob", Input: json.RawMessage(`{"pattern":"*.go"}`)},
				},
			},
			StopReason: llm.StopReasonToolUse,
			Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
		}
	}
	responses[5] = &llm.CompleteResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Content: `{"result": "done"}`},
		StopReason: llm.StopReasonEndTurn,
		Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
	}

	testTool := tool.NewToolFunc("glob", "Glob files", json.RawMessage(`{}`),
		func(_ context.Context, _ json.RawMessage) (string, error) { return "*.go", nil },
	)

	a := agent.NewAgent(&mockProvider{responses: responses},
		agent.WithTools(testTool),
		agent.WithMaxTurns(10),
	)

	output, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != `{"result": "done"}` {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestAgent_MaxTurnsReached(t *testing.T) {
	responses := make([]*llm.CompleteResponse, 10)
	for i := range responses {
		responses[i] = &llm.CompleteResponse{
			Message: llm.Message{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call", Name: "glob", Input: json.RawMessage(`{"pattern":"*.go"}`)},
				},
			},
			StopReason: llm.StopReasonToolUse,
			Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
		}
	}

	testTool := tool.NewToolFunc("glob", "Glob", json.RawMessage(`{}`),
		func(_ context.Context, _ json.RawMessage) (string, error) { return "ok", nil },
	)

	a := agent.NewAgent(&mockProvider{responses: responses},
		agent.WithTools(testTool),
		agent.WithMaxTurns(3),
	)

	_, err := a.Run(context.Background(), "Begin your task.")
	if err == nil {
		t.Fatal("expected error for max turns")
	}
	if !errors.Is(err, agent.ErrMaxTurns) {
		t.Errorf("expected ErrMaxTurns, got: %v", err)
	}
}

func TestAgent_ToolError(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "read_file", Input: json.RawMessage(`{"path":"bad.go"}`)},
					},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: `{"result": "recovered"}`},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
	}

	testTool := tool.NewToolFunc("read_file", "Read", json.RawMessage(`{}`),
		func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", errors.New("disk I/O failure")
		},
	)

	a := agent.NewAgent(provider, agent.WithTools(testTool), agent.WithMaxTurns(10))

	output, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("agent should continue after tool error, got: %v", err)
	}
	if output != `{"result": "recovered"}` {
		t.Errorf("expected recovered output, got %q", output)
	}
}

func TestAgent_ContextCancelled(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "should not reach"},
				StopReason: llm.StopReasonEndTurn,
			},
		},
	}

	a := agent.NewAgent(provider, agent.WithMaxTurns(10))

	_, err := a.Run(cancelCtx, "Begin your task.")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestAgent_SystemMessageContainsContext(t *testing.T) {
	var capturedSystem string
	provider := &capturingProvider{
		response: &llm.CompleteResponse{
			Message:    llm.Message{Role: llm.RoleAssistant, Content: `{"result": "done"}`},
			StopReason: llm.StopReasonEndTurn,
			Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
		},
		capturedSystem: &capturedSystem,
	}

	a := agent.NewAgent(provider,
		agent.WithSystemPrompt("Do the thing.\n\nbuild feature X"),
		agent.WithMaxTurns(10),
	)

	_, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedSystem == "" {
		t.Fatal("system message not captured")
	}
}

type capturingProvider struct {
	response       *llm.CompleteResponse
	capturedSystem *string
}

func (m *capturingProvider) Complete(_ context.Context, params llm.CompleteParams) (*llm.CompleteResponse, error) {
	*m.capturedSystem = params.SystemPrompt
	return m.response, nil
}
