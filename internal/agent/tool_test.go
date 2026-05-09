package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kilupskalvis/jerry/internal/agent"
)

func TestToolFunc_ImplementsTool(t *testing.T) {
	tf := agent.NewToolFunc(
		"test_tool",
		"A test tool",
		json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}},"required":["input"]}`),
		func(_ context.Context, input json.RawMessage) (string, error) {
			return "result", nil
		},
	)

	if tf.Name() != "test_tool" {
		t.Errorf("expected name 'test_tool', got %q", tf.Name())
	}
	if tf.Description() != "A test tool" {
		t.Errorf("expected description 'A test tool', got %q", tf.Description())
	}

	var schema map[string]any
	if err := json.Unmarshal(tf.Schema(), &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", schema["type"])
	}

	result, err := tf.Execute(context.Background(), json.RawMessage(`{"input":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "result" {
		t.Errorf("expected 'result', got %q", result)
	}
}

func TestToolsToDefinitions(t *testing.T) {
	tools := []agent.Tool{
		agent.NewToolFunc("a", "tool a", json.RawMessage(`{}`), nil),
		agent.NewToolFunc("b", "tool b", json.RawMessage(`{"type":"object"}`), nil),
	}

	defs := agent.ToolsToDefinitions(tools)
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	if defs[0].Name != "a" {
		t.Errorf("expected name 'a', got %q", defs[0].Name)
	}
	if defs[0].Description != "tool a" {
		t.Errorf("expected description 'tool a', got %q", defs[0].Description)
	}
	if defs[1].Name != "b" {
		t.Errorf("expected name 'b', got %q", defs[1].Name)
	}
}

func TestToolsToDefinitions_Empty(t *testing.T) {
	defs := agent.ToolsToDefinitions(nil)
	if len(defs) != 0 {
		t.Errorf("expected 0 definitions for nil input, got %d", len(defs))
	}
}
