package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/kilupskalvis/jerry/internal/agent"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

func anthropicResponse(content []map[string]any, stopReason string) string {
	resp := map[string]any{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"content":     content,
		"model":       "claude-sonnet-4-6",
		"stop_reason": stopReason,
		"usage": map[string]any{
			"input_tokens":  100,
			"output_tokens": 50,
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func textBlock(text string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": text,
	}
}

func toolUseBlock(id, name string, input map[string]any) map[string]any {
	return map[string]any{
		"type":  "tool_use",
		"id":    id,
		"name":  name,
		"input": input,
	}
}

func newTestAnthropicProvider(handler http.HandlerFunc) *agent.AnthropicProvider {
	server := httptest.NewServer(handler)
	return agent.NewAnthropicProvider("test-key",
		option.WithBaseURL(server.URL),
		option.WithMaxRetries(0),
	)
}

func TestAnthropicProvider_TextResponse(t *testing.T) {
	provider := newTestAnthropicProvider(func(w http.ResponseWriter, _ *http.Request) {
		body := anthropicResponse(
			[]map[string]any{textBlock("Hello, world!")},
			"end_turn",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	resp, err := provider.Complete(context.Background(), agent.CompleteParams{
		Model:        "claude-sonnet-4-6",
		SystemPrompt: "system prompt",
		Messages:     []agent.Message{{Role: agent.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %q", resp.Message.Content)
	}
	if len(resp.Message.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.Message.ToolCalls))
	}
	if resp.StopReason != agent.StopReasonEndTurn {
		t.Errorf("expected StopReasonEndTurn, got %q", resp.StopReason)
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", resp.Usage.OutputTokens)
	}
}

func TestAnthropicProvider_ToolCallResponse(t *testing.T) {
	provider := newTestAnthropicProvider(func(w http.ResponseWriter, _ *http.Request) {
		body := anthropicResponse(
			[]map[string]any{
				toolUseBlock("call_123", "read_file", map[string]any{"path": "main.go"}),
			},
			"tool_use",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	resp, err := provider.Complete(context.Background(), agent.CompleteParams{
		Model:    "claude-sonnet-4-6",
		Messages: []agent.Message{{Role: agent.RoleUser, Content: "Read the file"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	tc := resp.Message.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("expected tool call ID 'call_123', got %q", tc.ID)
	}
	if tc.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", tc.Name)
	}

	var args map[string]any
	if err := json.Unmarshal(tc.Input, &args); err != nil {
		t.Fatalf("failed to parse tool input: %v", err)
	}
	if args["path"] != "main.go" {
		t.Errorf("expected path 'main.go', got %v", args["path"])
	}
	if resp.StopReason != agent.StopReasonToolUse {
		t.Errorf("expected StopReasonToolUse, got %q", resp.StopReason)
	}
}

func TestAnthropicProvider_ToolResultCoalescing(t *testing.T) {
	var requestBody map[string]any

	provider := newTestAnthropicProvider(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &requestBody)
		body := anthropicResponse([]map[string]any{textBlock("Done")}, "end_turn")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	messages := []agent.Message{
		{Role: agent.RoleUser, Content: "Start"},
		{
			Role: agent.RoleAssistant,
			ToolCalls: []agent.ToolCall{
				{ID: "call_1", Name: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
				{ID: "call_2", Name: "read_file", Input: json.RawMessage(`{"path":"b.go"}`)},
			},
		},
		{Role: agent.RoleTool, ToolCallID: "call_1", Content: "content of a.go"},
		{Role: agent.RoleTool, ToolCallID: "call_2", Content: "content of b.go"},
	}

	_, err := provider.Complete(context.Background(), agent.CompleteParams{
		Model: "claude-sonnet-4-6", Messages: messages,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sdkMessages, ok := requestBody["messages"].([]any)
	if !ok {
		t.Fatal("messages not found in request body")
	}
	if len(sdkMessages) != 3 {
		t.Fatalf("expected 3 SDK messages, got %d", len(sdkMessages))
	}

	coalescedMsg := sdkMessages[2].(map[string]any)
	if coalescedMsg["role"] != "user" {
		t.Errorf("coalesced message should be role 'user', got %q", coalescedMsg["role"])
	}
	content := coalescedMsg["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks in coalesced message, got %d", len(content))
	}
	for i, block := range content {
		b := block.(map[string]any)
		if b["type"] != "tool_result" {
			t.Errorf("content block %d should be type 'tool_result', got %q", i, b["type"])
		}
	}
}

func TestAnthropicProvider_SystemPrompt(t *testing.T) {
	var requestBody map[string]any

	provider := newTestAnthropicProvider(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &requestBody)
		body := anthropicResponse([]map[string]any{textBlock("OK")}, "end_turn")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	_, err := provider.Complete(context.Background(), agent.CompleteParams{
		Model:        "claude-sonnet-4-6",
		SystemPrompt: "You are a test agent.",
		Messages:     []agent.Message{{Role: agent.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	system, ok := requestBody["system"].([]any)
	if !ok || len(system) == 0 {
		t.Fatal("system field not found or empty in request body")
	}
	firstBlock := system[0].(map[string]any)
	if firstBlock["text"] != "You are a test agent." {
		t.Errorf("expected system text 'You are a test agent.', got %q", firstBlock["text"])
	}
}

func TestAnthropicProvider_AuthFailure(t *testing.T) {
	provider := newTestAnthropicProvider(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	})

	_, err := provider.Complete(context.Background(), agent.CompleteParams{
		Model:    "claude-sonnet-4-6",
		Messages: []agent.Message{{Role: agent.RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}

	var jerryErr *jerrerr.Error
	if !errors.As(err, &jerryErr) {
		t.Fatalf("expected jerry Error, got %T: %v", err, err)
	}
	if jerryErr.Code != jerrerr.CodeLLMAuthFailed {
		t.Errorf("expected code %q, got %q", jerrerr.CodeLLMAuthFailed, jerryErr.Code)
	}
}

func TestAnthropicProvider_ContextCancelled(t *testing.T) {
	provider := newTestAnthropicProvider(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.Complete(cancelCtx, agent.CompleteParams{
		Model:    "claude-sonnet-4-6",
		Messages: []agent.Message{{Role: agent.RoleUser, Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestAnthropicProvider_EmptyToolsOmitted(t *testing.T) {
	var requestBody map[string]any

	provider := newTestAnthropicProvider(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &requestBody)
		body := anthropicResponse([]map[string]any{textBlock("OK")}, "end_turn")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	_, err := provider.Complete(context.Background(), agent.CompleteParams{
		Model:    "claude-sonnet-4-6",
		Messages: []agent.Message{{Role: agent.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := requestBody["tools"]; exists {
		t.Error("tools field should not be present when no tools are provided")
	}
}

func TestAnthropicProvider_ToolDefsTranslated(t *testing.T) {
	var requestBody map[string]any

	provider := newTestAnthropicProvider(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &requestBody)
		body := anthropicResponse([]map[string]any{textBlock("OK")}, "end_turn")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	tools := []agent.ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read a file",
			Schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path"}},"required":["path"]}`),
		},
	}

	_, err := provider.Complete(context.Background(), agent.CompleteParams{
		Model:    "claude-sonnet-4-6",
		Messages: []agent.Message{{Role: agent.RoleUser, Content: "Read"}},
		Tools:    tools,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sdkTools, ok := requestBody["tools"].([]any)
	if !ok || len(sdkTools) == 0 {
		t.Fatal("tools not found in request body")
	}
	tool := sdkTools[0].(map[string]any)
	if tool["name"] != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", tool["name"])
	}
	if !strings.Contains(tool["description"].(string), "Read a file") {
		t.Errorf("expected description containing 'Read a file', got %q", tool["description"])
	}
}

func TestAnthropicProvider_CacheTokensTracked(t *testing.T) {
	provider := newTestAnthropicProvider(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"id":          "msg_test",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]any{textBlock("OK")},
			"model":       "claude-sonnet-4-6",
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":                2500,
				"output_tokens":               750,
				"cache_creation_input_tokens": 1000,
				"cache_read_input_tokens":     500,
			},
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})

	resp, err := provider.Complete(context.Background(), agent.CompleteParams{
		Model:    "claude-sonnet-4-6",
		Messages: []agent.Message{{Role: agent.RoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Usage.InputTokens != 2500 {
		t.Errorf("expected 2500 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.CacheCreationTokens != 1000 {
		t.Errorf("expected 1000 cache creation tokens, got %d", resp.Usage.CacheCreationTokens)
	}
	if resp.Usage.CacheReadTokens != 500 {
		t.Errorf("expected 500 cache read tokens, got %d", resp.Usage.CacheReadTokens)
	}
}
