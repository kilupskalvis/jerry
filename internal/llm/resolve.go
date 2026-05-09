package llm

import (
	"fmt"
	"strings"
)

// ProviderResolver creates LLM providers from model names using configured API keys.
type ProviderResolver struct {
	keys map[string]string
}

func NewProviderResolver() *ProviderResolver {
	return &ProviderResolver{keys: make(map[string]string)}
}

func (r *ProviderResolver) SetKey(provider, key string) {
	if key != "" {
		r.keys[provider] = key
	}
}

// ForModel returns the appropriate Provider based on the model name prefix
// or an explicit provider override.
func (r *ProviderResolver) ForModel(model, provider string) (Provider, error) {
	if provider != "" {
		return r.forProvider(provider, model)
	}

	switch {
	case strings.HasPrefix(model, "claude-"):
		return r.forProvider("anthropic", model)
	case strings.HasPrefix(model, "gpt-"),
		strings.HasPrefix(model, "o1-"),
		strings.HasPrefix(model, "o3-"),
		strings.HasPrefix(model, "o4-"):
		return r.forProvider("openai", model)
	default:
		return nil, fmt.Errorf("unknown model provider for %q — "+
			"add a 'provider' field to the agent frontmatter (anthropic, openai)", model)
	}
}

func (r *ProviderResolver) forProvider(name, model string) (Provider, error) {
	key, ok := r.keys[name]
	if !ok || key == "" {
		envVar := providerEnvVar(name)
		return nil, fmt.Errorf("provider %q requires %s to be set", name, envVar)
	}

	switch name {
	case "anthropic":
		return NewAnthropicProvider(key), nil
	case "openai":
		return NewOpenAIProvider(key), nil
	default:
		return nil, fmt.Errorf("unknown provider %q (supported: anthropic, openai)", name)
	}
}

func providerEnvVar(name string) string {
	switch name {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	default:
		return strings.ToUpper(name) + "_API_KEY"
	}
}
