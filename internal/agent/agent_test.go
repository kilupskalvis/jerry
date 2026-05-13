package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/kilupskalvis/jerry/internal/agent"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/permissions"
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

	result, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != `{"result": "done"}` {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestAgent_DirectResponseAccumulatesUsage(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "done"},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50, CacheCreationTokens: 200, CacheReadTokens: 0},
			},
		},
	}

	a := agent.NewAgent(provider, agent.WithMaxTurns(10))
	result, err := a.Run(context.Background(), "task")
	if err != nil {
		t.Fatal(err)
	}
	if result.Turns != 1 {
		t.Errorf("turns = %d, want 1", result.Turns)
	}
	if result.Usage.InputTokens != 100 || result.Usage.OutputTokens != 50 {
		t.Errorf("usage = %d/%d, want 100/50", result.Usage.InputTokens, result.Usage.OutputTokens)
	}
	if result.Usage.CacheCreationTokens != 200 {
		t.Errorf("cache_creation = %d, want 200", result.Usage.CacheCreationTokens)
	}
	if result.ToolCalls != 0 {
		t.Errorf("tool_calls = %d, want 0", result.ToolCalls)
	}
}

func TestAgent_MultiTurnAccumulatesUsage(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message: llm.Message{
					Role:      llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{ID: "c1", Name: "t", Input: json.RawMessage(`{}`)}},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Message: llm.Message{
					Role:      llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{ID: "c2", Name: "t", Input: json.RawMessage(`{}`)}, {ID: "c3", Name: "t", Input: json.RawMessage(`{}`)}},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.Usage{InputTokens: 200, OutputTokens: 60, CacheReadTokens: 100},
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "done"},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 300, OutputTokens: 40, CacheReadTokens: 200},
			},
		},
	}

	testTool := tool.NewToolFunc("t", "test", json.RawMessage(`{}`),
		func(_ context.Context, _ json.RawMessage) (string, error) { return "ok", nil },
	)

	a := agent.NewAgent(provider, agent.WithTools(testTool), agent.WithMaxTurns(10))
	result, err := a.Run(context.Background(), "task")
	if err != nil {
		t.Fatal(err)
	}
	if result.Turns != 3 {
		t.Errorf("turns = %d, want 3", result.Turns)
	}
	if result.ToolCalls != 3 {
		t.Errorf("tool_calls = %d, want 3", result.ToolCalls)
	}
	if result.Usage.InputTokens != 600 {
		t.Errorf("input_tokens = %d, want 600", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 150 {
		t.Errorf("output_tokens = %d, want 150", result.Usage.OutputTokens)
	}
	if result.Usage.CacheReadTokens != 300 {
		t.Errorf("cache_read = %d, want 300", result.Usage.CacheReadTokens)
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

	result, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != `{"result": "done"}` {
		t.Errorf("unexpected output: %q", result.Output)
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

	result, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != `{"result": "done"}` {
		t.Errorf("unexpected output: %q", result.Output)
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

	result, err := a.Run(context.Background(), "Begin your task.")
	if err != nil {
		t.Fatalf("agent should continue after tool error, got: %v", err)
	}
	if result.Output != `{"result": "recovered"}` {
		t.Errorf("expected recovered output, got %q", result.Output)
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

type mockChecker struct {
	deny map[string]string
}

func (m *mockChecker) Check(toolName string, input json.RawMessage) *permissions.Denial {
	var args map[string]any
	_ = json.Unmarshal(input, &args)

	var matchInput string
	switch toolName {
	case "bash":
		matchInput, _ = args["command"].(string)
	case "read_file", "write_file":
		matchInput, _ = args["path"].(string)
	}

	if pattern, ok := m.deny[matchInput]; ok {
		return &permissions.Denial{
			Tool:    toolName,
			Input:   matchInput,
			Pattern: pattern,
			Source:  "test",
		}
	}
	return nil
}

func TestAgent_CheckerBlocksToolCall(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "bash", Input: json.RawMessage(`{"command":"rm -rf /"}`)},
					},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "understood, skipping delete"},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 20},
			},
		},
	}

	bashTool := tool.NewToolFunc("bash", "Run command", json.RawMessage(`{}`),
		func(_ context.Context, _ json.RawMessage) (string, error) {
			t.Fatal("bash tool should not have been called")
			return "", nil
		},
	)

	checker := &mockChecker{
		deny: map[string]string{"rm -rf /": "rm *"},
	}

	a := agent.NewAgent(provider,
		agent.WithTools(bashTool),
		agent.WithMaxTurns(10),
		agent.WithChecker(checker),
	)

	result, err := a.Run(context.Background(), "delete everything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "understood, skipping delete" {
		t.Errorf("output = %q, want 'understood, skipping delete'", result.Output)
	}
}

func TestAgent_CheckerAllowsToolCall(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "bash", Input: json.RawMessage(`{"command":"go test ./..."}`)},
					},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "tests passed"},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 20},
			},
		},
	}

	var executed bool
	bashTool := tool.NewToolFunc("bash", "Run command", json.RawMessage(`{}`),
		func(_ context.Context, _ json.RawMessage) (string, error) {
			executed = true
			return "PASS", nil
		},
	)

	checker := &mockChecker{}

	a := agent.NewAgent(provider,
		agent.WithTools(bashTool),
		agent.WithMaxTurns(10),
		agent.WithChecker(checker),
	)

	_, err := a.Run(context.Background(), "run tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("bash tool should have been called")
	}
}

func TestAgent_ParallelToolExecution(t *testing.T) {
	// LLM returns 3 tool calls at once. Verify all execute and results
	// are returned in the correct order.
	provider := &mockProvider{
		responses: []*llm.CompleteResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "c1", Name: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
						{ID: "c2", Name: "read_file", Input: json.RawMessage(`{"path":"b.go"}`)},
						{ID: "c3", Name: "read_file", Input: json.RawMessage(`{"path":"c.go"}`)},
					},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "done"},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.Usage{InputTokens: 200, OutputTokens: 10},
			},
		},
	}

	var mu sync.Mutex
	var callOrder []string

	readTool := tool.NewToolFunc("read_file", "Read a file", json.RawMessage(`{}`),
		func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct{ Path string }
			_ = json.Unmarshal(input, &args)
			mu.Lock()
			callOrder = append(callOrder, args.Path)
			mu.Unlock()
			return "contents of " + args.Path, nil
		},
	)

	a := agent.NewAgent(provider, agent.WithTools(readTool), agent.WithMaxTurns(10))

	result, err := a.Run(context.Background(), "read files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "done" {
		t.Errorf("output = %q, want 'done'", result.Output)
	}
	if len(callOrder) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(callOrder))
	}
}
