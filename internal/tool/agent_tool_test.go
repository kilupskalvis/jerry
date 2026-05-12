package tool_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/tool"
)

func TestAgentTool_Name(t *testing.T) {
	t.Parallel()
	at := tool.NewAgentTool("security_reviewer", "Check for vulnerabilities.", nil)
	if at.Name() != "security_reviewer" {
		t.Errorf("name = %q, want 'security_reviewer'", at.Name())
	}
}

func TestAgentTool_Description(t *testing.T) {
	t.Parallel()
	at := tool.NewAgentTool("security_reviewer", "Check for vulnerabilities.\nMore details here.", nil)
	desc := at.Description()
	if !strings.Contains(desc, "security_reviewer") {
		t.Errorf("description should contain agent name, got %q", desc)
	}
	if !strings.Contains(desc, "Check for vulnerabilities.") {
		t.Errorf("description should contain first line, got %q", desc)
	}
	if strings.Contains(desc, "More details") {
		t.Errorf("description should only contain first line, got %q", desc)
	}
}

func TestAgentTool_Schema(t *testing.T) {
	t.Parallel()
	at := tool.NewAgentTool("test", "Test.", nil)
	schema := at.Schema()

	var s map[string]any
	if err := json.Unmarshal(schema, &s); err != nil {
		t.Fatalf("invalid schema JSON: %v", err)
	}

	props, _ := s["properties"].(map[string]any)
	if _, ok := props["task"]; !ok {
		t.Error("schema missing 'task' property")
	}

	required, _ := s["required"].([]any)
	if len(required) != 1 || required[0] != "task" {
		t.Errorf("required = %v, want [task]", required)
	}
}

func TestAgentTool_Execute(t *testing.T) {
	t.Parallel()
	at := tool.NewAgentTool("security_reviewer", "Check for vulnerabilities.",
		func(_ context.Context, task string) (string, error) {
			return "No vulnerabilities found.", nil
		},
	)

	input, _ := json.Marshal(map[string]string{"task": "Review auth.go"})
	result, err := at.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No vulnerabilities found." {
		t.Errorf("result = %q, want 'No vulnerabilities found.'", result)
	}
}

func TestAgentTool_ExecutePassesTask(t *testing.T) {
	t.Parallel()
	var receivedTask string
	at := tool.NewAgentTool("scanner", "Scan.",
		func(_ context.Context, task string) (string, error) {
			receivedTask = task
			return "done", nil
		},
	)

	input, _ := json.Marshal(map[string]string{"task": "Check main.go for XSS"})
	at.Execute(context.Background(), input) //nolint:errcheck // test
	if receivedTask != "Check main.go for XSS" {
		t.Errorf("task = %q, want 'Check main.go for XSS'", receivedTask)
	}
}

func TestAgentTool_EmptyTask(t *testing.T) {
	t.Parallel()
	at := tool.NewAgentTool("test", "Test.", nil)

	input, _ := json.Marshal(map[string]string{"task": ""})
	result, _ := at.Execute(context.Background(), input)
	if !strings.Contains(result, "missing required") {
		t.Errorf("got %q, want missing required error", result)
	}
}

func TestAgentTool_RunFuncError(t *testing.T) {
	t.Parallel()
	at := tool.NewAgentTool("broken", "Broken agent.",
		func(_ context.Context, _ string) (string, error) {
			return "", fmt.Errorf("agent crashed")
		},
	)

	input, _ := json.Marshal(map[string]string{"task": "do something"})
	result, err := at.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute should not return Go error, got: %v", err)
	}
	if !strings.Contains(result, "Subagent") && !strings.Contains(result, "failed") {
		t.Errorf("got %q, want error message about subagent failure", result)
	}
}
