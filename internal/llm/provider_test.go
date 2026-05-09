package llm_test

import (
	"testing"

	"github.com/kilupskalvis/jerry/internal/llm"
)

func TestNewProviderForModel_Claude(t *testing.T) {
	provider, err := llm.NewProviderForModel("claude-sonnet-4-6", "", "ant-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.AnthropicProvider); !ok {
		t.Errorf("expected AnthropicProvider, got %T", provider)
	}
}

func TestNewProviderForModel_GPT(t *testing.T) {
	provider, err := llm.NewProviderForModel("gpt-4o", "", "", "oai-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.OpenAIProvider); !ok {
		t.Errorf("expected OpenAIProvider, got %T", provider)
	}
}

func TestNewProviderForModel_O1(t *testing.T) {
	provider, err := llm.NewProviderForModel("o1-preview", "", "", "oai-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.OpenAIProvider); !ok {
		t.Errorf("expected OpenAIProvider, got %T", provider)
	}
}

func TestNewProviderForModel_O3(t *testing.T) {
	provider, err := llm.NewProviderForModel("o3-mini", "", "", "oai-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.OpenAIProvider); !ok {
		t.Errorf("expected OpenAIProvider, got %T", provider)
	}
}

func TestNewProviderForModel_Unknown(t *testing.T) {
	_, err := llm.NewProviderForModel("llama-3", "", "", "")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestNewProviderForModel_UnknownWithProvider(t *testing.T) {
	provider, err := llm.NewProviderForModel("my-fine-tuned", "openai", "", "oai-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := provider.(*llm.OpenAIProvider); !ok {
		t.Errorf("expected OpenAIProvider, got %T", provider)
	}
}

func TestNewProviderForModel_MissingAnthropicKey(t *testing.T) {
	_, err := llm.NewProviderForModel("claude-sonnet-4-6", "", "", "")
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY")
	}
}

func TestNewProviderForModel_MissingOpenAIKey(t *testing.T) {
	_, err := llm.NewProviderForModel("gpt-4o", "", "", "")
	if err == nil {
		t.Fatal("expected error for missing OPENAI_API_KEY")
	}
}
