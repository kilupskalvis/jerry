// Provider interface and types for provider-agnostic LLM communication.

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

// ToolsToDefinitions extracts LLM-facing metadata from a slice of Tools.
func ToolsToDefinitions(tools []Tool) []ToolDefinition {
	defs := make([]ToolDefinition, len(tools))
	for i, t := range tools {
		defs[i] = ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		}
	}
	return defs
}

// NewProviderForModel returns the appropriate Provider based on the model name
// prefix or an explicit provider override.
func NewProviderForModel(model, provider, anthropicKey, openaiKey string) (Provider, error) {
	if provider != "" {
		switch provider {
		case "anthropic":
			if anthropicKey == "" {
				return nil, fmt.Errorf("provider 'anthropic' requires ANTHROPIC_API_KEY to be set")
			}
			return NewAnthropicProvider(anthropicKey), nil
		case "openai":
			if openaiKey == "" {
				return nil, fmt.Errorf("provider 'openai' requires OPENAI_API_KEY to be set")
			}
			return NewOpenAIProvider(openaiKey), nil
		default:
			return nil, fmt.Errorf("unknown provider %q (supported: anthropic, openai)", provider)
		}
	}

	switch {
	case strings.HasPrefix(model, "claude-"):
		if anthropicKey == "" {
			return nil, fmt.Errorf("model %q requires ANTHROPIC_API_KEY to be set", model)
		}
		return NewAnthropicProvider(anthropicKey), nil

	case strings.HasPrefix(model, "gpt-"),
		strings.HasPrefix(model, "o1-"),
		strings.HasPrefix(model, "o3-"),
		strings.HasPrefix(model, "o4-"):
		if openaiKey == "" {
			return nil, fmt.Errorf("model %q requires OPENAI_API_KEY to be set", model)
		}
		return NewOpenAIProvider(openaiKey), nil

	default:
		return nil, fmt.Errorf("unknown model provider for %q — "+
			"add a 'provider' field to the agent frontmatter (anthropic, openai)", model)
	}
}
