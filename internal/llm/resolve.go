package llm

import (
	"fmt"
	"strings"
)

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
