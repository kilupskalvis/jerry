package tool_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/tool"
)

func TestBash_Success(t *testing.T) {
	t.Parallel()
	bash := tool.NewBashTool(t.TempDir(), nil)
	input, _ := json.Marshal(map[string]string{"command": "echo hello"})
	result, err := bash.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("got %q, want output containing 'hello'", result)
	}
}

func TestBash_StderrCaptured(t *testing.T) {
	t.Parallel()
	bash := tool.NewBashTool(t.TempDir(), nil)
	input, _ := json.Marshal(map[string]string{"command": "echo err >&2"})
	result, err := bash.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "err") {
		t.Errorf("stderr not captured, got %q", result)
	}
}

func TestBash_FailureReturnsExitCode(t *testing.T) {
	t.Parallel()
	bash := tool.NewBashTool(t.TempDir(), nil)
	input, _ := json.Marshal(map[string]string{"command": "exit 42"})
	result, err := bash.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "exit code 42") {
		t.Errorf("got %q, want exit code 42", result)
	}
}

func TestBash_CleanEnv(t *testing.T) {
	t.Parallel()
	bash := tool.NewBashTool(t.TempDir(), []string{"JERRY_SECRET_FOO=bar"})
	input, _ := json.Marshal(map[string]string{"command": "echo $JERRY_SECRET_FOO"})
	result, err := bash.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "bar") {
		t.Errorf("secret not available, got %q", result)
	}
}

func TestBash_NoLeakProcessEnv(t *testing.T) {
	t.Setenv("SHOULD_NOT_LEAK", "secret123")
	bash := tool.NewBashTool(t.TempDir(), nil)
	input, _ := json.Marshal(map[string]string{"command": "echo $SHOULD_NOT_LEAK"})
	result, err := bash.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "secret123") {
		t.Error("process env leaked to bash tool")
	}
}

func TestBash_EmptyCommand(t *testing.T) {
	t.Parallel()
	bash := tool.NewBashTool(t.TempDir(), nil)
	input, _ := json.Marshal(map[string]string{"command": ""})
	result, err := bash.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "missing required") {
		t.Errorf("got %q, want missing required error", result)
	}
}

func TestBash_WorkingDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bash := tool.NewBashTool(dir, nil)
	input, _ := json.Marshal(map[string]string{"command": "pwd"})
	result, err := bash.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, dir) {
		t.Errorf("got %q, want working dir %q", result, dir)
	}
}
