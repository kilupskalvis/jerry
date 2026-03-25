package script_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/contextstore"
	"github.com/kilupskalvis/jerry/internal/pipeline"
	"github.com/kilupskalvis/jerry/internal/script"
)

func newTestExecutor(t *testing.T) (*script.Executor, string) {
	t.Helper()
	repoRoot := t.TempDir()
	env := map[string]string{
		"JERRY_SECRET_TEST": "secret_value",
	}
	return script.NewExecutor(repoRoot, env), repoRoot
}

func newTestStore() *contextstore.Store {
	return contextstore.NewStore("run_test", contextstore.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: "test",
	})
}

func TestCanExecute(t *testing.T) {
	exec, _ := newTestExecutor(t)

	if !exec.CanExecute(pipeline.Step{Script: "echo hi"}) {
		t.Error("should handle steps with Script set")
	}
	if exec.CanExecute(pipeline.Step{Agent: "./agent.md"}) {
		t.Error("should not handle steps with Agent set")
	}
	if exec.CanExecute(pipeline.Step{}) {
		t.Error("should not handle empty steps")
	}
}

func TestExecute_Success(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{Name: "test", Script: "echo hello"}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", output.ExitCode)
	}
	if strings.TrimSpace(output.Stdout) != "hello" {
		t.Errorf("Stdout = %q, want %q", strings.TrimSpace(output.Stdout), "hello")
	}
}

func TestExecute_Failure(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{Name: "test", Script: "exit 1"}

	stepCtx := context.Background()
	_, err := exec.Execute(stepCtx, step, store)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "SCRIPT_FAILED") {
		t.Errorf("error should contain SCRIPT_FAILED, got %q", err.Error())
	}
}

func TestExecute_JSONOutput(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{
		Name:      "test",
		Script:    `echo '{"key":"value"}'`,
		OutputKey: "result",
	}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Data == nil {
		t.Fatal("Data should be parsed JSON, got nil")
	}
	dataMap, ok := output.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data should be a map, got %T", output.Data)
	}
	if dataMap["key"] != "value" {
		t.Errorf("Data[key] = %v, want %q", dataMap["key"], "value")
	}
}

func TestExecute_InvalidJSON_StillSucceeds(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{
		Name:      "test",
		Script:    "echo 'not json'",
		OutputKey: "result",
	}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("step should succeed even with invalid JSON output, got error: %v", err)
	}
	if output.Data != nil {
		t.Errorf("Data should be nil when stdout is not valid JSON, got %v", output.Data)
	}
}

func TestExecute_Environment(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{Name: "test", Script: "env | sort"}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(output.Stdout, "\n")
	envMap := make(map[string]string)
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Should have JERRY_* vars
	if _, ok := envMap["JERRY_RUN_ID"]; !ok {
		t.Error("missing JERRY_RUN_ID")
	}
	if _, ok := envMap["JERRY_STEP_NAME"]; !ok {
		t.Error("missing JERRY_STEP_NAME")
	}
	if _, ok := envMap["JERRY_CONTEXT_FILE"]; !ok {
		t.Error("missing JERRY_CONTEXT_FILE")
	}
	if _, ok := envMap["PATH"]; !ok {
		t.Error("missing PATH")
	}
	if _, ok := envMap["HOME"]; !ok {
		t.Error("missing HOME")
	}

	// Should NOT have random env vars from the parent process
	// (checking a few common ones that should be excluded)
	for key := range envMap {
		if key == "PATH" || key == "HOME" || strings.HasPrefix(key, "JERRY_") {
			continue
		}
		// Allow PWD and SHLVL which sh may set internally
		if key == "PWD" || key == "SHLVL" || key == "_" {
			continue
		}
		t.Errorf("unexpected env var %q should not be in clean environment", key)
	}
}

func TestExecute_SecretEnvVars(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{Name: "test", Script: "echo $JERRY_SECRET_TEST"}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(output.Stdout) != "secret_value" {
		t.Errorf("Stdout = %q, want %q", strings.TrimSpace(output.Stdout), "secret_value")
	}
}

func TestExecute_ContextFile(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	_ = store.Set("test_data", "hello")

	step := pipeline.Step{Name: "test", Script: `cat "$JERRY_CONTEXT_FILE"`}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stdout should be valid JSON matching the context
	var parsed contextstore.Context
	if jsonErr := json.Unmarshal([]byte(output.Stdout), &parsed); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", jsonErr, output.Stdout)
	}
	if parsed.RunID != "run_test" {
		t.Errorf("RunID = %q, want %q", parsed.RunID, "run_test")
	}
	if parsed.Data["test_data"] != "hello" {
		t.Errorf("Data[test_data] = %v, want %q", parsed.Data["test_data"], "hello")
	}
}

func TestExecute_Timeout(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{Name: "test", Script: "sleep 60"}

	stepCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := exec.Execute(stepCtx, step, store)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "SCRIPT_TIMEOUT") {
		t.Errorf("error should contain SCRIPT_TIMEOUT, got %q", err.Error())
	}
}

func TestExecute_ProcessGroupKill(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	// Script spawns a background child — both should be killed
	step := pipeline.Step{Name: "test", Script: "sleep 3600 & sleep 60"}

	stepCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := exec.Execute(stepCtx, step, store)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	// The test passing without hanging proves the process group was killed
}

func TestExecute_WorkingDirectory(t *testing.T) {
	exec, repoRoot := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{Name: "test", Script: "pwd"}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// pwd output should match repo root
	got := strings.TrimSpace(output.Stdout)

	// Resolve both to handle symlinks (macOS /tmp → /private/tmp)
	resolvedGot, _ := filepath.EvalSymlinks(got)
	resolvedExpected, _ := filepath.EvalSymlinks(repoRoot)

	if resolvedGot != resolvedExpected {
		t.Errorf("working directory = %q, want %q", resolvedGot, resolvedExpected)
	}
}

func TestExecute_StderrCapture(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{Name: "test", Script: "echo error_msg >&2"}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output.Stderr, "error_msg") {
		t.Errorf("Stderr = %q, should contain %q", output.Stderr, "error_msg")
	}
}

func TestExecute_StdoutStderrSeparation(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{Name: "test", Script: `echo "out_data"; echo "err_data" >&2`}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output.Stdout, "out_data") {
		t.Errorf("Stdout should contain 'out_data', got %q", output.Stdout)
	}
	if !strings.Contains(output.Stderr, "err_data") {
		t.Errorf("Stderr should contain 'err_data', got %q", output.Stderr)
	}
	// Stdout should NOT contain stderr content
	if strings.Contains(output.Stdout, "err_data") {
		t.Error("Stdout should not contain stderr content")
	}
}

func TestExecute_NoOutputKey(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	step := pipeline.Step{
		Name:   "test",
		Script: `echo '{"key":"value"}'`,
		// No OutputKey set
	}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Data != nil {
		t.Errorf("Data should be nil when no output_key is set, got %v", output.Data)
	}
	// But stdout should still be captured
	if !strings.Contains(output.Stdout, "key") {
		t.Error("Stdout should still be captured even without output_key")
	}
}

func TestExecute_ContextFileCleanup(t *testing.T) {
	exec, _ := newTestExecutor(t)
	store := newTestStore()
	// Script prints the context file path so we can check it was cleaned up
	step := pipeline.Step{Name: "test", Script: `echo "$JERRY_CONTEXT_FILE"`}

	stepCtx := context.Background()
	output, err := exec.Execute(stepCtx, step, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contextFilePath := strings.TrimSpace(output.Stdout)
	if _, statErr := os.Stat(contextFilePath); !os.IsNotExist(statErr) {
		t.Errorf("context file should be cleaned up after execution, still exists at %q", contextFilePath)
	}
}
