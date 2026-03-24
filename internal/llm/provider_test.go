package llm

import "testing"

func TestNewClientForModel_Claude(t *testing.T) {
	client, err := NewClientForModel("claude-sonnet-4-6", "", "ant-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.(*AnthropicClient); !ok {
		t.Errorf("expected AnthropicClient, got %T", client)
	}
}

func TestNewClientForModel_GPT(t *testing.T) {
	client, err := NewClientForModel("gpt-4o", "", "", "oai-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("expected OpenAIClient, got %T", client)
	}
}

func TestNewClientForModel_O1(t *testing.T) {
	client, err := NewClientForModel("o1-preview", "", "", "oai-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("expected OpenAIClient, got %T", client)
	}
}

func TestNewClientForModel_O3(t *testing.T) {
	client, err := NewClientForModel("o3-mini", "", "", "oai-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("expected OpenAIClient, got %T", client)
	}
}

func TestNewClientForModel_Unknown(t *testing.T) {
	_, err := NewClientForModel("llama-3", "", "", "")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestNewClientForModel_UnknownWithProvider(t *testing.T) {
	client, err := NewClientForModel("my-fine-tuned", "openai", "", "oai-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("expected OpenAIClient, got %T", client)
	}
}

func TestNewClientForModel_MissingAnthropicKey(t *testing.T) {
	_, err := NewClientForModel("claude-sonnet-4-6", "", "", "")
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY")
	}
}

func TestNewClientForModel_MissingOpenAIKey(t *testing.T) {
	_, err := NewClientForModel("gpt-4o", "", "", "")
	if err == nil {
		t.Fatal("expected error for missing OPENAI_API_KEY")
	}
}
