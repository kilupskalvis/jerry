// Package llm defines the provider-neutral interface for language model communication.
package llm

import (
	"context"
	"errors"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message represents a single message in the LLM conversation.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolID    string     `json:"tool_id,omitempty"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolDef describes a tool available to the LLM.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// Response holds the model's response to a Send call.
type Response struct {
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason"`
	Usage      TokenUsage `json:"usage"`
}

// TokenUsage tracks token consumption for a single LLM call.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Client communicates with a language model provider.
type Client interface {
	Send(ctx context.Context, system string, messages []Message, tools []ToolDef) (*Response, error)
}

// ContextTooLongError indicates the conversation exceeded the model's context window.
type ContextTooLongError struct {
	Message string
}

func (e *ContextTooLongError) Error() string {
	return e.Message
}

// IsContextTooLong checks if err is a ContextTooLongError.
func IsContextTooLong(err error) bool {
	if err == nil {
		return false
	}
	var ctxErr *ContextTooLongError
	return errors.As(err, &ctxErr)
}
