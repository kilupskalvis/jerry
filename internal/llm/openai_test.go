package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/option"
)

func newTestOpenAIClient(serverURL string) *OpenAIClient {
	return NewOpenAIClient("test-key", "gpt-4o", option.WithBaseURL(serverURL))
}

func TestOpenAI_TextResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "Hello from GPT"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     100,
				"completion_tokens": 20,
				"total_tokens":      120,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)
	resp, err := client.Send(context.Background(), "system prompt",
		[]Message{{Role: RoleUser, Content: "hello"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from GPT" {
		t.Errorf("content = %q, want %q", resp.Content, "Hello from GPT")
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want %q", resp.StopReason, "end_turn")
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("input_tokens = %d, want 100", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 20 {
		t.Errorf("output_tokens = %d, want 20", resp.Usage.OutputTokens)
	}
}

func TestOpenAI_ToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]any{
									"name":      "read_file",
									"arguments": `{"path":"test.go"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]any{
				"prompt_tokens": 150, "completion_tokens": 30, "total_tokens": 180,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)
	resp, err := client.Send(context.Background(), "system",
		[]Message{{Role: RoleUser, Content: "read the file"}},
		[]ToolDef{{Name: "read_file", Description: "Read a file"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool_calls length = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("tool name = %q, want %q", resp.ToolCalls[0].Name, "read_file")
	}
	if resp.ToolCalls[0].ID != "call_123" {
		t.Errorf("tool id = %q, want %q", resp.ToolCalls[0].ID, "call_123")
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("stop_reason = %q, want %q", resp.StopReason, "tool_use")
	}
}

func TestOpenAI_SystemAsMessage(t *testing.T) {
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

	client := newTestOpenAIClient(server.URL)
	_, _ = client.Send(context.Background(), "You are helpful",
		[]Message{{Role: RoleUser, Content: "hi"}}, nil)

	messages, ok := receivedBody["messages"].([]any)
	if !ok || len(messages) == 0 {
		t.Fatal("expected messages in request body")
	}
	firstMsg := messages[0].(map[string]any)
	if firstMsg["role"] != "system" {
		t.Errorf("first message role = %v, want 'system'", firstMsg["role"])
	}
}

func TestOpenAI_ContextTooLong(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		resp := map[string]any{
			"error": map[string]any{
				"message": "This model's maximum context length is 128000 tokens",
				"type":    "invalid_request_error",
				"code":    "context_length_exceeded",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)
	_, err := client.Send(context.Background(), "system",
		[]Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error for context too long")
	}
	if !IsContextTooLong(err) {
		t.Errorf("IsContextTooLong = false, want true for: %v", err)
	}
}

func TestOpenAI_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		resp := map[string]any{
			"error": map[string]any{"message": "Incorrect API key"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)
	_, err := client.Send(context.Background(), "system",
		[]Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected auth error")
	}
}
