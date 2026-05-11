// Package llm provides a provider-agnostic abstraction for LLM communication.
package llm

import (
	"context"
	"encoding/json"
	"errors"
)

// Provider abstracts an LLM completion API. Implementations translate between
// the provider-agnostic types in this package and vendor-specific wire formats.
type Provider interface {
	// Complete sends a conversation with tool definitions to the LLM and returns a response.
	Complete(ctx context.Context, params CompleteParams) (*CompleteResponse, error)
}

// CompleteParams configures a single LLM completion request.
type CompleteParams struct {
	// Model identifies which LLM model to use.
	Model string
	// SystemPrompt is the system-level instruction for the LLM.
	SystemPrompt string
	// Messages is the conversation history.
	Messages []Message
	// Tools describes the tools available for this request.
	Tools []ToolDefinition
	// MaxTokens caps the response length. Zero uses the provider's default.
	MaxTokens int
	// Temperature controls randomness. Nil uses the provider's default.
	Temperature *float64
}

// ToolDefinition is the LLM-facing metadata for a tool, without execution capability.
// Providers pass this to the LLM so it knows which tools are available.
type ToolDefinition struct {
	// Name uniquely identifies the tool.
	Name string `json:"name"`
	// Description explains when and how the LLM should use this tool.
	Description string `json:"description"`
	// Schema is the JSON Schema for the tool's input parameters.
	Schema json.RawMessage `json:"input_schema"`
}

// CompleteResponse holds the result of an LLM completion.
type CompleteResponse struct {
	// Message is the assistant's response, which may contain text, tool calls, or both.
	Message Message
	// Usage reports token consumption for this request.
	Usage Usage
	// StopReason indicates why the LLM stopped generating.
	StopReason StopReason
}

// Usage reports token consumption for a single LLM request.
type Usage struct {
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}

// StopReason indicates why the LLM stopped generating.
type StopReason string

const (
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonMaxTokens StopReason = "max_tokens"
)

// ContextTooLongError indicates the conversation exceeded the model's context window.
type ContextTooLongError struct {
	Message string
}

func (e *ContextTooLongError) Error() string {
	return e.Message
}

// IsContextTooLong checks if err is a ContextTooLongError.
func IsContextTooLong(err error) bool {
	var ctxErr *ContextTooLongError
	return errors.As(err, &ctxErr)
}
