// Provider selection: maps model names to the correct LLM client.

package llm

import (
	"fmt"
	"strings"
)

// NewClientForModel returns the appropriate LLM client based on the model name
// prefix or an explicit provider override.
func NewClientForModel(model, provider, anthropicKey, openaiKey string) (Client, error) {
	// Explicit provider override takes precedence.
	if provider != "" {
		switch provider {
		case "anthropic":
			if anthropicKey == "" {
				return nil, fmt.Errorf("provider 'anthropic' requires ANTHROPIC_API_KEY to be set")
			}
			return NewAnthropicClient(anthropicKey, model), nil
		case "openai":
			if openaiKey == "" {
				return nil, fmt.Errorf("provider 'openai' requires OPENAI_API_KEY to be set")
			}
			return NewOpenAIClient(openaiKey, model), nil
		default:
			return nil, fmt.Errorf("unknown provider %q (supported: anthropic, openai)", provider)
		}
	}

	// Prefix-based detection.
	switch {
	case strings.HasPrefix(model, "claude-"):
		if anthropicKey == "" {
			return nil, fmt.Errorf("model %q requires ANTHROPIC_API_KEY to be set", model)
		}
		return NewAnthropicClient(anthropicKey, model), nil

	case strings.HasPrefix(model, "gpt-"),
		strings.HasPrefix(model, "o1-"),
		strings.HasPrefix(model, "o3-"),
		strings.HasPrefix(model, "o4-"):
		if openaiKey == "" {
			return nil, fmt.Errorf("model %q requires OPENAI_API_KEY to be set", model)
		}
		return NewOpenAIClient(openaiKey, model), nil

	default:
		return nil, fmt.Errorf("unknown model provider for %q — "+
			"add a 'provider' field to the agent frontmatter (anthropic, openai)", model)
	}
}
