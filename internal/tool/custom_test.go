package tool_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/tool"
)

func writeToolYAML(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCustomTool_NoParams(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")

	writeToolYAML(t, toolsDir, "greet", `
description: Say hello
run: echo "hello world"
`)

	tools, err := tool.LoadCustomTools(toolsDir, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
	if tools[0].Name() != "greet" {
		t.Errorf("name = %q, want 'greet'", tools[0].Name())
	}

	result, err := tools[0].Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("got %q, want 'hello world'", result)
	}
}

func TestCustomTool_WithParams(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")

	writeToolYAML(t, toolsDir, "deploy", `
description: Deploy a service
parameters:
  service:
    type: string
    required: true
run: echo "deploying $TOOL_SERVICE"
`)

	tools, err := tool.LoadCustomTools(toolsDir, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	input, _ := json.Marshal(map[string]string{"service": "auth-api"})
	result, err := tools[0].Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result, "deploying auth-api") {
		t.Errorf("got %q, want 'deploying auth-api'", result)
	}
}

func TestCustomTool_HyphenParam(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")

	writeToolYAML(t, toolsDir, "notify", `
description: Send notification
parameters:
  channel-name:
    type: string
run: echo "$TOOL_CHANNEL_NAME"
`)

	tools, err := tool.LoadCustomTools(toolsDir, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	input, _ := json.Marshal(map[string]string{"channel-name": "alerts"})
	result, err := tools[0].Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result, "alerts") {
		t.Errorf("got %q, want 'alerts'", result)
	}
}

func TestCustomTool_SecretEnv(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")

	writeToolYAML(t, toolsDir, "api", `
description: Call API
run: echo "$JERRY_SECRET_TOKEN"
`)

	tools, err := tool.LoadCustomTools(toolsDir, t.TempDir(), []string{"JERRY_SECRET_TOKEN=abc123"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := tools[0].Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result, "abc123") {
		t.Errorf("got %q, want 'abc123'", result)
	}
}

func TestCustomTool_StdinReceivesJSON(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")

	writeToolYAML(t, toolsDir, "echo-stdin", `
description: Echo stdin
run: cat
`)

	tools, err := tool.LoadCustomTools(toolsDir, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	input := json.RawMessage(`{"key": "value"}`)
	result, err := tools[0].Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result, `"key"`) {
		t.Errorf("got %q, want JSON on stdin", result)
	}
}

func TestCustomTool_FailureReturnsError(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")

	writeToolYAML(t, toolsDir, "fail", `
description: Always fails
run: exit 1
`)

	tools, err := tool.LoadCustomTools(toolsDir, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result, err := tools[0].Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !strings.Contains(result, "Error") {
		t.Errorf("got %q, want error result", result)
	}
}

func TestCustomTool_MissingDescription(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")

	writeToolYAML(t, toolsDir, "bad", `run: echo hi`)

	_, err := tool.LoadCustomTools(toolsDir, t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for missing description")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("got %q, want error about description", err.Error())
	}
}

func TestCustomTool_MissingRun(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")

	writeToolYAML(t, toolsDir, "bad", `description: Does nothing`)

	_, err := tool.LoadCustomTools(toolsDir, t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for missing run")
	}
	if !strings.Contains(err.Error(), "run") {
		t.Errorf("got %q, want error about run", err.Error())
	}
}

func TestCustomTool_EmptyDir(t *testing.T) {
	t.Parallel()
	tools, err := tool.LoadCustomTools(filepath.Join(t.TempDir(), "nonexistent"), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("got %d tools, want 0", len(tools))
	}
}
