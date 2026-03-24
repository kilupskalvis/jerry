// Package llm defines the provider-neutral interface for communicating with
// language models. Concrete implementations (Anthropic, OpenAI) translate
// these types to and from provider-specific API formats.
//
// The Client interface is the only dependency that the agent package has on
// this package — everything else (tool layer, agentic loop) uses these types
// directly without knowing which provider is behind them.
package llm

import (
	"context"
	"errors"
)

// Role constants for messages.
// RoleSystem is unused in Phase 2 (Anthropic passes system separately)
// but included for Phase 3 OpenAI compatibility where system is a message.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Message represents a single message in the LLM conversation.
type Message struct {
	// Role is one of the Role* constants.
	Role string `json:"role"`

	// Content is the text content of the message. Empty for assistant
	// messages that only contain tool calls.
	Content string `json:"content,omitempty"`

	// ToolCalls holds tool invocations requested by the assistant.
	// Only populated when Role == RoleAssistant.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolID links a tool result message back to the tool call that
	// produced it. Only populated when Role == RoleTool.
	ToolID string `json:"tool_id,omitempty"`
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
	// Content is the text portion of the response. May be empty when the
	// model responds with only tool calls.
	Content string `json:"content,omitempty"`

	// ToolCalls holds tool invocations requested by the model.
	// Non-empty when StopReason is "tool_use".
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// StopReason indicates why the model stopped generating.
	// Common values: "end_turn" (model finished), "tool_use" (wants to call tools).
	StopReason string `json:"stop_reason"`

	// Usage tracks token consumption for this single call.
	Usage TokenUsage `json:"usage"`
}

// TokenUsage tracks token consumption for a single LLM call.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Client communicates with a language model provider.
// Implementations handle provider-specific API formats, retry logic,
// and error translation. The agentic loop uses this interface to
// remain provider-agnostic.
type Client interface {
	// Send sends messages to the LLM and returns the response.
	// The system prompt is passed separately from conversation messages
	// because some providers (Anthropic) require this separation.
	// tools defines which tools are available for this call; pass nil
	// for no tool access.
	Send(requestCtx context.Context, system string, messages []Message, tools []ToolDef) (*Response, error)
}

// ContextTooLongError indicates the conversation exceeded the model's context window.
// Each provider client wraps provider-specific detection behind this type so the
// agentic loop can trigger compaction.
type ContextTooLongError struct {
	Message string
}

func (e *ContextTooLongError) Error() string {
	return e.Message
}

// IsContextTooLong returns true if the error indicates the conversation
// exceeded the model's context window.
func IsContextTooLong(err error) bool {
	if err == nil {
		return false
	}
	var ctxErr *ContextTooLongError
	return errors.As(err, &ctxErr)
}
