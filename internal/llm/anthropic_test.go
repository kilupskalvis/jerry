package llm_test

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

	jerryErrors "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/llm"
)

// anthropicResponse builds a minimal Anthropic API response JSON.
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

func newTestClient(handler http.HandlerFunc) *llm.AnthropicClient {
	server := httptest.NewServer(handler)
	return llm.NewAnthropicClient("test-key", "claude-sonnet-4-6",
		option.WithBaseURL(server.URL),
		option.WithMaxRetries(0), // Disable retries for predictable test behavior
	)
}

func TestAnthropicClient_TextResponse(t *testing.T) {
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		body := anthropicResponse(
			[]map[string]any{textBlock("Hello, world!")},
			"end_turn",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	resp, err := client.Send(context.Background(), "system prompt", []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("expected stop_reason 'end_turn', got %q", resp.StopReason)
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", resp.Usage.OutputTokens)
	}
}

func TestAnthropicClient_ToolCallResponse(t *testing.T) {
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		body := anthropicResponse(
			[]map[string]any{
				toolUseBlock("call_123", "read_file", map[string]any{
					"path": "main.go",
				}),
			},
			"tool_use",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	resp, err := client.Send(context.Background(), "", []llm.Message{
		{Role: llm.RoleUser, Content: "Read the file"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("expected tool call ID 'call_123', got %q", tc.ID)
	}
	if tc.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", tc.Name)
	}
	if tc.Arguments["path"] != "main.go" {
		t.Errorf("expected path 'main.go', got %v", tc.Arguments["path"])
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("expected stop_reason 'tool_use', got %q", resp.StopReason)
	}
}

func TestAnthropicClient_MultipleToolCalls(t *testing.T) {
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		body := anthropicResponse(
			[]map[string]any{
				toolUseBlock("call_1", "read_file", map[string]any{"path": "a.go"}),
				toolUseBlock("call_2", "glob", map[string]any{"pattern": "*.go"}),
				toolUseBlock("call_3", "read_file", map[string]any{"path": "b.go"}),
			},
			"tool_use",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	resp, err := client.Send(context.Background(), "", []llm.Message{
		{Role: llm.RoleUser, Content: "Read files"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("first call should be read_file, got %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[1].Name != "glob" {
		t.Errorf("second call should be glob, got %q", resp.ToolCalls[1].Name)
	}
}

func TestAnthropicClient_ToolResultCoalescing(t *testing.T) {
	var requestBody map[string]any

	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &requestBody)

		body := anthropicResponse(
			[]map[string]any{textBlock("Done")},
			"end_turn",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	// Send an assistant message with 2 tool calls, followed by 2 tool results.
	// The tool results should be coalesced into a single user message.
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Start"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call_1", Name: "read_file", Arguments: map[string]any{"path": "a.go"}},
				{ID: "call_2", Name: "read_file", Arguments: map[string]any{"path": "b.go"}},
			},
		},
		{Role: llm.RoleTool, ToolID: "call_1", Content: "content of a.go"},
		{Role: llm.RoleTool, ToolID: "call_2", Content: "content of b.go"},
	}

	_, err := client.Send(context.Background(), "", messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request body: the 2 tool result messages should be coalesced
	// into a single user message with 2 tool_result content blocks.
	sdkMessages, ok := requestBody["messages"].([]any)
	if !ok {
		t.Fatal("messages not found in request body")
	}

	// Expected: user, assistant, user (coalesced tool results)
	if len(sdkMessages) != 3 {
		t.Fatalf("expected 3 SDK messages (user, assistant, coalesced user), got %d", len(sdkMessages))
	}

	// Third message should be a user message with 2 tool_result content blocks.
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

func TestAnthropicClient_SystemPrompt(t *testing.T) {
	var requestBody map[string]any

	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &requestBody)

		body := anthropicResponse(
			[]map[string]any{textBlock("OK")},
			"end_turn",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	_, err := client.Send(context.Background(), "You are a test agent.", []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// System prompt should be passed as a separate "system" field, not as a message.
	system, ok := requestBody["system"].([]any)
	if !ok || len(system) == 0 {
		t.Fatal("system field not found or empty in request body")
	}

	firstBlock := system[0].(map[string]any)
	if firstBlock["text"] != "You are a test agent." {
		t.Errorf("expected system text 'You are a test agent.', got %q", firstBlock["text"])
	}

	// Verify system is NOT included as a message.
	messages := requestBody["messages"].([]any)
	for _, msg := range messages {
		m := msg.(map[string]any)
		if m["role"] == "system" {
			t.Error("system prompt should not be a message with role 'system'")
		}
	}
}

func TestAnthropicClient_AuthFailure(t *testing.T) {
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	})

	_, err := client.Send(context.Background(), "", []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	}, nil)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}

	var jerryErr *jerryErrors.Error
	if !errors.As(err, &jerryErr) {
		t.Fatalf("expected jerry Error, got %T: %v", err, err)
	}
	if jerryErr.Code != jerryErrors.CodeLLMAuthFailed {
		t.Errorf("expected code %q, got %q", jerryErrors.CodeLLMAuthFailed, jerryErr.Code)
	}
}

func TestAnthropicClient_ContextCancelled(t *testing.T) {
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		// Never respond — the client should cancel first.
		<-r.Context().Done()
	})

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.Send(cancelCtx, "", []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	}, nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestAnthropicClient_EmptyToolsOmitted(t *testing.T) {
	var requestBody map[string]any

	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &requestBody)

		body := anthropicResponse(
			[]map[string]any{textBlock("OK")},
			"end_turn",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	_, err := client.Send(context.Background(), "", []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When no tools are provided, the "tools" field should not be in the request.
	if _, exists := requestBody["tools"]; exists {
		t.Error("tools field should not be present when no tools are provided")
	}
}

func TestAnthropicClient_ToolDefsTranslated(t *testing.T) {
	var requestBody map[string]any

	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		json.Unmarshal(bodyBytes, &requestBody)

		body := anthropicResponse(
			[]map[string]any{textBlock("OK")},
			"end_turn",
		)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	tools := []llm.ToolDef{
		{
			Name:        "read_file",
			Description: "Read a file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "File path",
					},
				},
				"required": []any{"path"},
			},
		},
	}

	_, err := client.Send(context.Background(), "", []llm.Message{
		{Role: llm.RoleUser, Content: "Read"},
	}, tools)
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

func TestAnthropicClient_TokenUsage(t *testing.T) {
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id":          "msg_test",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]any{textBlock("OK")},
			"model":       "claude-sonnet-4-6",
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":  2500,
				"output_tokens": 750,
			},
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})

	resp, err := client.Send(context.Background(), "", []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Usage.InputTokens != 2500 {
		t.Errorf("expected 2500 input tokens, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 750 {
		t.Errorf("expected 750 output tokens, got %d", resp.Usage.OutputTokens)
	}
}
