package hooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/hooks"
)

func TestRunner_FireSimple(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	h := hooks.Hooks{
		hooks.OnWorkflowComplete: {
			{Run: "echo done > " + outFile},
		},
	}

	runner := hooks.NewRunner(h, dir, nil)
	runner.Fire(hooks.OnWorkflowComplete, nil)

	time.Sleep(50 * time.Millisecond)
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("hook did not write file: %v", err)
	}
	if !strings.Contains(string(data), "done") {
		t.Errorf("got %q, want 'done'", string(data))
	}
}

func TestRunner_FireWithEnvVars(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	h := hooks.Hooks{
		hooks.OnWorkflowComplete: {
			{Run: "echo $JERRY_HOOK_STATUS > " + outFile},
		},
	}

	runner := hooks.NewRunner(h, dir, nil)
	runner.Fire(hooks.OnWorkflowComplete, map[string]string{
		"JERRY_HOOK_STATUS": "completed",
	})

	time.Sleep(50 * time.Millisecond)
	data, _ := os.ReadFile(outFile)
	if !strings.Contains(string(data), "completed") {
		t.Errorf("got %q, want 'completed'", string(data))
	}
}

func TestRunner_FireWithBaseEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	h := hooks.Hooks{
		hooks.OnWorkflowStart: {
			{Run: "echo $JERRY_HOOK_WORKFLOW > " + outFile},
		},
	}

	runner := hooks.NewRunner(h, dir, nil)
	runner.SetBaseEnv(map[string]string{
		"JERRY_HOOK_WORKFLOW": "review",
	})
	runner.Fire(hooks.OnWorkflowStart, nil)

	time.Sleep(50 * time.Millisecond)
	data, _ := os.ReadFile(outFile)
	if !strings.Contains(string(data), "review") {
		t.Errorf("got %q, want 'review'", string(data))
	}
}

func TestRunner_ToolFilter_Matches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	h := hooks.Hooks{
		hooks.BeforeToolCall: {
			{Tools: []string{"write_file"}, Run: "echo matched > " + outFile},
		},
	}

	runner := hooks.NewRunner(h, dir, nil)
	runner.Fire(hooks.BeforeToolCall, map[string]string{
		"JERRY_HOOK_TOOL_NAME": "write_file",
	})

	time.Sleep(50 * time.Millisecond)
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("filtered hook should have fired: %v", err)
	}
	if !strings.Contains(string(data), "matched") {
		t.Errorf("got %q, want 'matched'", string(data))
	}
}

func TestRunner_ToolFilter_NoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	h := hooks.Hooks{
		hooks.BeforeToolCall: {
			{Tools: []string{"write_file"}, Run: "echo should-not-run > " + outFile},
		},
	}

	runner := hooks.NewRunner(h, dir, nil)
	runner.Fire(hooks.BeforeToolCall, map[string]string{
		"JERRY_HOOK_TOOL_NAME": "read_file",
	})

	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(outFile); err == nil {
		t.Error("hook should not have fired for non-matching tool")
	}
}

func TestRunner_ToolFilter_NoFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	h := hooks.Hooks{
		hooks.AfterToolCall: {
			{Run: "echo any-tool > " + outFile},
		},
	}

	runner := hooks.NewRunner(h, dir, nil)
	runner.Fire(hooks.AfterToolCall, map[string]string{
		"JERRY_HOOK_TOOL_NAME": "bash",
	})

	time.Sleep(50 * time.Millisecond)
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("unfiltered hook should fire for any tool: %v", err)
	}
	if !strings.Contains(string(data), "any-tool") {
		t.Errorf("got %q, want 'any-tool'", string(data))
	}
}

func TestRunner_MultipleHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	h := hooks.Hooks{
		hooks.OnWorkflowComplete: {
			{Run: "echo first >> " + outFile},
			{Run: "echo second >> " + outFile},
		},
	}

	runner := hooks.NewRunner(h, dir, nil)
	runner.Fire(hooks.OnWorkflowComplete, nil)

	time.Sleep(50 * time.Millisecond)
	data, _ := os.ReadFile(outFile)
	if !strings.Contains(string(data), "first") || !strings.Contains(string(data), "second") {
		t.Errorf("both hooks should fire, got %q", string(data))
	}
}

func TestRunner_NoEvent(t *testing.T) {
	t.Parallel()
	h := hooks.Hooks{}
	runner := hooks.NewRunner(h, t.TempDir(), nil)
	runner.Fire(hooks.OnWorkflowComplete, nil)
}

func TestRunner_NilRunner(t *testing.T) {
	t.Parallel()
	var runner *hooks.Runner
	runner.Fire(hooks.OnWorkflowComplete, nil)
}

func TestRunner_FailedHookDoesNotPanic(t *testing.T) {
	t.Parallel()
	h := hooks.Hooks{
		hooks.OnWorkflowComplete: {
			{Run: "exit 1"},
		},
	}
	runner := hooks.NewRunner(h, t.TempDir(), nil)
	runner.Fire(hooks.OnWorkflowComplete, nil)
}

func TestRunner_SecretEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	h := hooks.Hooks{
		hooks.OnWorkflowComplete: {
			{Run: "echo $JERRY_SECRET_TOKEN > " + outFile},
		},
	}

	runner := hooks.NewRunner(h, dir, []string{"JERRY_SECRET_TOKEN=abc123"})
	runner.Fire(hooks.OnWorkflowComplete, nil)

	time.Sleep(50 * time.Millisecond)
	data, _ := os.ReadFile(outFile)
	if !strings.Contains(string(data), "abc123") {
		t.Errorf("secret not available, got %q", string(data))
	}
}
