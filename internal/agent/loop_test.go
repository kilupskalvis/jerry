package agent_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kilupskalvis/motif/internal/agent"
	motifErrors "github.com/kilupskalvis/motif/internal/errors"
	"github.com/kilupskalvis/motif/internal/llm"
	"github.com/kilupskalvis/motif/internal/output"
)

func newTestLoop(responses []*llm.Response) (*agent.Loop, *mockLLMClient) {
	client := &mockLLMClient{responses: responses}
	printer := output.NewPrinter(devNull{}, devNull{})
	return agent.NewLoop(client, nil, printer), client
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }

func testConfig() agent.AgentConfig {
	return agent.AgentConfig{
		Name:          "test-agent",
		MaxIterations: 10,
		OutputSchema:  map[string]any{"result": "string"},
	}
}

func noopDispatch(_ context.Context, call llm.ToolCall) (string, error) {
	return "tool result for " + call.Name, nil
}

func TestLoop_DirectResponse(t *testing.T) {
	loop, _ := newTestLoop([]*llm.Response{
		{
			Content:    `{"result": "done"}`,
			StopReason: "end_turn",
			Usage:      llm.TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
	})

	result, err := loop.Run(context.Background(), testConfig(), nil, noopDispatch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", result.Iterations)
	}
	if result.RawOutput != `{"result": "done"}` {
		t.Errorf("unexpected raw output: %q", result.RawOutput)
	}
	if result.ToolCalls != 0 {
		t.Errorf("expected 0 tool calls, got %d", result.ToolCalls)
	}
}

func TestLoop_OneToolCall(t *testing.T) {
	loop, _ := newTestLoop([]*llm.Response{
		{
			StopReason: "tool_use",
			ToolCalls: []llm.ToolCall{
				{ID: "call_1", Name: "read_file", Arguments: map[string]any{"path": "main.go"}},
			},
			Usage: llm.TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
		{
			Content:    `{"result": "done"}`,
			StopReason: "end_turn",
			Usage:      llm.TokenUsage{InputTokens: 200, OutputTokens: 30},
		},
	})

	var dispatchedCalls []string
	dispatch := func(_ context.Context, call llm.ToolCall) (string, error) {
		dispatchedCalls = append(dispatchedCalls, call.Name)
		return "file contents", nil
	}

	result, err := loop.Run(context.Background(), testConfig(), nil, dispatch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Iterations != 2 {
		t.Errorf("expected 2 iterations, got %d", result.Iterations)
	}
	if result.ToolCalls != 1 {
		t.Errorf("expected 1 tool call, got %d", result.ToolCalls)
	}
	if len(dispatchedCalls) != 1 || dispatchedCalls[0] != "read_file" {
		t.Errorf("expected dispatched [read_file], got %v", dispatchedCalls)
	}
}

func TestLoop_MultipleIterations(t *testing.T) {
	responses := make([]*llm.Response, 6)
	for i := 0; i < 5; i++ {
		responses[i] = &llm.Response{
			StopReason: "tool_use",
			ToolCalls: []llm.ToolCall{
				{ID: "call", Name: "glob", Arguments: map[string]any{"pattern": "*.go"}},
			},
			Usage: llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
		}
	}
	responses[5] = &llm.Response{
		Content:    `{"result": "done"}`,
		StopReason: "end_turn",
		Usage:      llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
	}

	loop, _ := newTestLoop(responses)
	result, err := loop.Run(context.Background(), testConfig(), nil, noopDispatch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Iterations != 6 {
		t.Errorf("expected 6 iterations, got %d", result.Iterations)
	}
	if result.ToolCalls != 5 {
		t.Errorf("expected 5 tool calls, got %d", result.ToolCalls)
	}
}

func TestLoop_MaxIterationsReached(t *testing.T) {
	config := testConfig()
	config.MaxIterations = 3

	// All responses have tool calls — loop never gets a text-only response.
	responses := make([]*llm.Response, 10)
	for i := range responses {
		responses[i] = &llm.Response{
			StopReason: "tool_use",
			ToolCalls: []llm.ToolCall{
				{ID: "call", Name: "glob", Arguments: map[string]any{"pattern": "*.go"}},
			},
			Usage: llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
		}
	}

	loop, _ := newTestLoop(responses)
	_, err := loop.Run(context.Background(), config, nil, noopDispatch, nil)
	if err == nil {
		t.Fatal("expected error for max iterations")
	}

	var motifErr *motifErrors.Error
	if !errors.As(err, &motifErr) {
		t.Fatalf("expected motif Error, got %T: %v", err, err)
	}
	if motifErr.Code != motifErrors.CodeAgentMaxIterations {
		t.Errorf("expected code %q, got %q", motifErrors.CodeAgentMaxIterations, motifErr.Code)
	}
}

func TestLoop_ToolError(t *testing.T) {
	loop, _ := newTestLoop([]*llm.Response{
		{
			StopReason: "tool_use",
			ToolCalls: []llm.ToolCall{
				{ID: "call_1", Name: "read_file", Arguments: map[string]any{"path": "bad.go"}},
			},
			Usage: llm.TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
		{
			Content:    `{"result": "recovered"}`,
			StopReason: "end_turn",
			Usage:      llm.TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
	})

	dispatch := func(_ context.Context, _ llm.ToolCall) (string, error) {
		return "", errors.New("disk I/O failure")
	}

	result, err := loop.Run(context.Background(), testConfig(), nil, dispatch, nil)
	if err != nil {
		t.Fatalf("loop should continue after tool error, got: %v", err)
	}
	if result.RawOutput != `{"result": "recovered"}` {
		t.Errorf("expected recovered output, got %q", result.RawOutput)
	}
}

func TestLoop_ContextCancelled(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	loop, _ := newTestLoop([]*llm.Response{
		{Content: "should not reach", StopReason: "end_turn"},
	})

	_, err := loop.Run(cancelCtx, testConfig(), nil, noopDispatch, nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestLoop_TokenAccumulation(t *testing.T) {
	loop, _ := newTestLoop([]*llm.Response{
		{
			StopReason: "tool_use",
			ToolCalls:  []llm.ToolCall{{ID: "c1", Name: "glob"}},
			Usage:      llm.TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
		{
			StopReason: "tool_use",
			ToolCalls:  []llm.ToolCall{{ID: "c2", Name: "glob"}},
			Usage:      llm.TokenUsage{InputTokens: 200, OutputTokens: 60},
		},
		{
			Content:    `{"result": "done"}`,
			StopReason: "end_turn",
			Usage:      llm.TokenUsage{InputTokens: 150, OutputTokens: 40},
		},
	})

	result, err := loop.Run(context.Background(), testConfig(), nil, noopDispatch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalUsage.InputTokens != 450 {
		t.Errorf("expected 450 input tokens, got %d", result.TotalUsage.InputTokens)
	}
	if result.TotalUsage.OutputTokens != 150 {
		t.Errorf("expected 150 output tokens, got %d", result.TotalUsage.OutputTokens)
	}
}

func TestLoop_SystemMessageContainsContext(t *testing.T) {
	var capturedSystem string
	client := &capturingMockClient{
		response: &llm.Response{
			Content:    `{"result": "done"}`,
			StopReason: "end_turn",
			Usage:      llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
		capturedSystem: &capturedSystem,
	}

	printer := output.NewPrinter(devNull{}, devNull{})
	loop := agent.NewLoop(client, nil, printer)

	config := testConfig()
	config.Instructions = "# Test Agent\n\nDo the thing."

	pipelineContext := map[string]any{
		"trigger": map[string]any{
			"intent": "build feature X",
		},
	}

	_, err := loop.Run(context.Background(), config, nil, noopDispatch, pipelineContext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedSystem, "Do the thing") {
		t.Error("system message should contain agent instructions")
	}
	if !strings.Contains(capturedSystem, "build feature X") {
		t.Error("system message should contain pipeline context")
	}
}

func TestLoop_SystemMessageContainsSchema(t *testing.T) {
	var capturedSystem string
	client := &capturingMockClient{
		response: &llm.Response{
			Content:    `{"result": "done"}`,
			StopReason: "end_turn",
			Usage:      llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
		capturedSystem: &capturedSystem,
	}

	printer := output.NewPrinter(devNull{}, devNull{})
	loop := agent.NewLoop(client, nil, printer)

	config := testConfig()
	config.Instructions = "# Agent"
	config.OutputSchema = map[string]any{
		"summary":    "string",
		"confidence": "number",
	}

	_, err := loop.Run(context.Background(), config, nil, noopDispatch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedSystem, "Output Format") {
		t.Error("system message should contain output format section")
	}
	if !strings.Contains(capturedSystem, "summary") {
		t.Error("system message should contain schema keys")
	}
}
