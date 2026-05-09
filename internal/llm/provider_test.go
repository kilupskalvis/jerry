package llm_test

import (
	"testing"

	"github.com/kilupskalvis/jerry/internal/llm"
)

func newResolver(anthropicKey, openaiKey string) *llm.ProviderResolver {
	r := llm.NewProviderResolver()
	r.SetKey("anthropic", anthropicKey)
	r.SetKey("openai", openaiKey)
	return r
}

func TestForModel_Claude(t *testing.T) {
	provider, err := newResolver("ant-key", "").ForModel("claude-sonnet-4-6", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.AnthropicProvider); !ok {
		t.Errorf("expected AnthropicProvider, got %T", provider)
	}
}

func TestForModel_GPT(t *testing.T) {
	provider, err := newResolver("", "oai-key").ForModel("gpt-4o", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.OpenAIProvider); !ok {
		t.Errorf("expected OpenAIProvider, got %T", provider)
	}
}

func TestForModel_O1(t *testing.T) {
	provider, err := newResolver("", "oai-key").ForModel("o1-preview", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.OpenAIProvider); !ok {
		t.Errorf("expected OpenAIProvider, got %T", provider)
	}
}

func TestForModel_O3(t *testing.T) {
	provider, err := newResolver("", "oai-key").ForModel("o3-mini", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.OpenAIProvider); !ok {
		t.Errorf("expected OpenAIProvider, got %T", provider)
	}
}

func TestForModel_Unknown(t *testing.T) {
	_, err := newResolver("", "").ForModel("llama-3", "")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestForModel_ExplicitProvider(t *testing.T) {
	provider, err := newResolver("", "oai-key").ForModel("my-fine-tuned", "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.OpenAIProvider); !ok {
		t.Errorf("expected OpenAIProvider, got %T", provider)
	}
}

func TestForModel_MissingAnthropicKey(t *testing.T) {
	_, err := newResolver("", "").ForModel("claude-sonnet-4-6", "")
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY")
	}
}

func TestForModel_MissingOpenAIKey(t *testing.T) {
	_, err := newResolver("", "").ForModel("gpt-4o", "")
	if err == nil {
		t.Fatal("expected error for missing OPENAI_API_KEY")
	}
}
