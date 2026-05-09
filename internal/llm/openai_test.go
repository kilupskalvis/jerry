package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/option"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/llm"
)

func newTestOpenAIProvider(serverURL string) *llm.OpenAIProvider {
	return llm.NewOpenAIProvider("test-key", option.WithBaseURL(serverURL))
}

func TestOpenAIProvider_TextResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 1234567890, "model": "gpt-4o",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]any{"role": "assistant", "content": "Hello from GPT"}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := newTestOpenAIProvider(server.URL)
	resp, err := provider.Complete(context.Background(), llm.CompleteParams{
		Model: "gpt-4o", SystemPrompt: "system prompt",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "Hello from GPT" {
		t.Errorf("content = %q, want %q", resp.Message.Content, "Hello from GPT")
	}
	if resp.StopReason != llm.StopReasonEndTurn {
		t.Errorf("stop_reason = %q, want %q", resp.StopReason, llm.StopReasonEndTurn)
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("input_tokens = %d, want 100", resp.Usage.InputTokens)
	}
}

func TestOpenAIProvider_ToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 1234567890, "model": "gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role": "assistant", "content": "",
						"tool_calls": []map[string]any{
							{"id": "call_123", "type": "function", "function": map[string]any{"name": "read_file", "arguments": `{"path":"test.go"}`}},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]any{"prompt_tokens": 150, "completion_tokens": 30, "total_tokens": 180},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := newTestOpenAIProvider(server.URL)
	resp, err := provider.Complete(context.Background(), llm.CompleteParams{
		Model: "gpt-4o", Messages: []llm.Message{{Role: llm.RoleUser, Content: "read the file"}},
		Tools: []llm.ToolDefinition{{Name: "read_file", Description: "Read a file", Schema: json.RawMessage(`{}`)}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls length = %d, want 1", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Name != "read_file" {
		t.Errorf("tool name = %q, want %q", resp.Message.ToolCalls[0].Name, "read_file")
	}
	if resp.StopReason != llm.StopReasonToolUse {
		t.Errorf("stop_reason = %q, want %q", resp.StopReason, llm.StopReasonToolUse)
	}
}

func TestOpenAIProvider_SystemAsMessage(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		resp := map[string]any{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 1234567890, "model": "gpt-4o",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := newTestOpenAIProvider(server.URL)
	_, _ = provider.Complete(context.Background(), llm.CompleteParams{
		Model: "gpt-4o", SystemPrompt: "You are helpful",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})

	messages, ok := receivedBody["messages"].([]any)
	if !ok || len(messages) == 0 {
		t.Fatal("expected messages in request body")
	}
	firstMsg := messages[0].(map[string]any)
	if firstMsg["role"] != "system" {
		t.Errorf("first message role = %v, want 'system'", firstMsg["role"])
	}
}

func TestOpenAIProvider_ContextTooLong(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "This model's maximum context length is 128000 tokens",
				"type":    "invalid_request_error",
				"code":    "context_length_exceeded",
			},
		})
	}))
	defer server.Close()

	provider := newTestOpenAIProvider(server.URL)
	_, err := provider.Complete(context.Background(), llm.CompleteParams{
		Model: "gpt-4o", Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for context too long")
	}
	if !llm.IsContextTooLong(err) {
		t.Errorf("IsContextTooLong = false, want true for: %v", err)
	}
}

func TestOpenAIProvider_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "Incorrect API key"}})
	}))
	defer server.Close()

	provider := newTestOpenAIProvider(server.URL)
	_, err := provider.Complete(context.Background(), llm.CompleteParams{
		Model: "gpt-4o", Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected auth error")
	}

	var jerryErr *jerrerr.Error
	if !errors.As(err, &jerryErr) {
		t.Fatalf("expected jerry Error, got %T: %v", err, err)
	}
	if jerryErr.Code != jerrerr.CodeLLMAuthFailed {
		t.Errorf("expected code %q, got %q", jerrerr.CodeLLMAuthFailed, jerryErr.Code)
	}
}
