// Package agent implements the StepExecutor for AI agent steps.
// In Phase 1, this is a stub that skips agent steps with a warning.
// Phase 2 provides the full implementation with the agentic loop.
package agent

import "context"

// Message represents a single message in the LLM conversation.
type Message struct {
	Role      string     `json:"role"`                // "system", "user", "assistant", "tool"
	Content   string     `json:"content,omitempty"`   // text content
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // tool calls made by the assistant
	ToolID    string     `json:"tool_id,omitempty"`    // tool result ID (for role "tool")
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
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// LLMResponse holds the model's response.
type LLMResponse struct {
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

// LLMClient communicates with a language model provider.
// Defined in Phase 1 so the contract is established. Implementations
// are added in Phase 2 (Anthropic) and Phase 3 (OpenAI).
type LLMClient interface {
	// Send sends messages to the LLM and returns the response.
	// The system prompt is passed separately from the conversation messages.
	// Tools defines which tools are available for this call.
	Send(requestCtx context.Context, system string, messages []Message, tools []ToolDef) (*LLMResponse, error)
}
