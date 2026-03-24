// Package integration contains end-to-end tests that exercise the full
// Motif pipeline: loading, validation, execution, state persistence,
// context flow, retries, fallbacks, and timeouts.
//
// These tests construct real pipeline YAMLs, run the engine with real
// script executors, and verify the results — no mocks.
package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kilupskalvis/motif/internal/agent"
	"github.com/kilupskalvis/motif/internal/config"
	"github.com/kilupskalvis/motif/internal/contextstore"
	"github.com/kilupskalvis/motif/internal/output"
	"github.com/kilupskalvis/motif/internal/pipeline"
	"github.com/kilupskalvis/motif/internal/script"
	"github.com/kilupskalvis/motif/internal/state"
	"github.com/kilupskalvis/motif/internal/testutil"
	"github.com/kilupskalvis/motif/internal/tools"
)

// buildEngine constructs a full engine with real executors for integration testing.
func buildEngine(t *testing.T, repoRoot string) (*pipeline.Engine, *state.FileStateStore) {
	t.Helper()

	motifDir := filepath.Join(repoRoot, ".motif")
	runsDir := filepath.Join(motifDir, "runs")
	if mkErr := os.MkdirAll(runsDir, 0o755); mkErr != nil {
		t.Fatalf("failed to create runs dir: %v", mkErr)
	}

	stateStore := state.NewFileStateStore(runsDir)
	printer := output.NewPrinter(devNull{}, devNull{})
	scriptExec := script.NewExecutor(repoRoot, map[string]string{})

	toolRegistry := tools.NewRegistry(repoRoot, nil)
	agentLoader := agent.NewLoader(toolRegistry.KnownToolNames(), "")
	agentExec := agent.NewExecutor(agentLoader, toolRegistry, nil, printer)

	engine := pipeline.NewEngine(
		[]pipeline.StepExecutor{agentExec, scriptExec},
		stateStore,
		printer,
		config.DefaultStepTimeoutValue,
	)

	return engine, stateStore
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }

// --- Tests ---

func TestFullPipeline_Init(t *testing.T) {
	// Simulate what motif init creates, then validate
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"example": `name: example
description: "Example pipeline"
steps:
  - name: show-trigger
    script: |
      echo "Pipeline triggered"
      cat "$MOTIF_CONTEXT_FILE"
  - name: list-files
    script: ls -la
  - name: check-status
    script: |
      echo '{"status": "ok"}'
    output_key: result
`,
	})

	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	// Validate should pass
	p, loadErr := loader.Load("example")
	if loadErr != nil {
		t.Fatalf("validation failed: %v", loadErr)
	}
	if p.Name != "example" {
		t.Errorf("Name = %q, want %q", p.Name, "example")
	}
	if len(p.Steps) != 3 {
		t.Errorf("Steps = %d, want 3", len(p.Steps))
	}
}

func TestFullPipeline_RunExample(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"example": `name: example
steps:
  - name: greet
    script: echo "hello motif"
  - name: status
    script: |
      echo '{"status": "ok"}'
    output_key: result
`,
	})

	engine, stateStore := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: "integration test",
	}

	runCtx := context.Background()
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("example")

	runState, runErr := engine.Run(runCtx, *p, trigger)
	if runErr != nil {
		t.Fatalf("pipeline run failed: %v", runErr)
	}

	if runState.Status != state.StatusCompleted {
		t.Errorf("Status = %q, want completed", runState.Status)
	}
	if len(runState.StepResults) != 2 {
		t.Fatalf("StepResults = %d, want 2", len(runState.StepResults))
	}
	for _, sr := range runState.StepResults {
		if sr.Status != state.StepSuccess {
			t.Errorf("step %q status = %q, want success", sr.Name, sr.Status)
		}
	}

	// Verify state files exist
	runsDir := filepath.Join(motifDir, "runs")
	entries, _ := os.ReadDir(runsDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 run directory, got %d", len(entries))
	}

	runDir := filepath.Join(runsDir, entries[0].Name())
	if _, statErr := os.Stat(filepath.Join(runDir, "state.json")); statErr != nil {
		t.Error("state.json should exist")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "log.json")); statErr != nil {
		t.Error("log.json should exist")
	}

	// Verify state can be loaded back
	loaded, loadErr := stateStore.LoadRun(runState.RunID)
	if loadErr != nil {
		t.Fatalf("failed to load run state: %v", loadErr)
	}
	if loaded.Status != state.StatusCompleted {
		t.Errorf("loaded Status = %q, want completed", loaded.Status)
	}
}

func TestFullPipeline_ContextFlow(t *testing.T) {
	// Step 1 produces JSON output, step 2 reads it from context file
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"flow": `name: flow
steps:
  - name: produce
    script: |
      echo '{"produced_value": "hello_from_step_one"}'
    output_key: step_one_data
  - name: consume
    script: |
      cat "$MOTIF_CONTEXT_FILE"
    output_key: step_two_data
`,
	})

	engine, _ := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}

	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("flow")

	runState, runErr := engine.Run(context.Background(), *p, trigger)
	if runErr != nil {
		t.Fatalf("pipeline run failed: %v", runErr)
	}

	// The context should contain step_one_data
	stepOneData, ok := runState.Context.Data["step_one_data"]
	if !ok {
		t.Fatal("context should contain 'step_one_data'")
	}

	dataMap, ok := stepOneData.(map[string]any)
	if !ok {
		t.Fatalf("step_one_data should be a map, got %T", stepOneData)
	}
	if dataMap["produced_value"] != "hello_from_step_one" {
		t.Errorf("produced_value = %v, want 'hello_from_step_one'", dataMap["produced_value"])
	}

	// Step 2's output should contain the context with step_one_data in it
	// (it ran cat $MOTIF_CONTEXT_FILE which includes step_one_data)
	stepTwoData, ok := runState.Context.Data["step_two_data"]
	if !ok {
		t.Fatal("context should contain 'step_two_data'")
	}
	stepTwoMap, ok := stepTwoData.(map[string]any)
	if !ok {
		t.Fatalf("step_two_data should be a map, got %T", stepTwoData)
	}

	// The context file content (parsed by step 2) should have the data map
	// containing step_one_data
	if dataField, hasData := stepTwoMap["data"]; hasData {
		dataFieldMap, ok := dataField.(map[string]any)
		if ok {
			if _, hasStepOne := dataFieldMap["step_one_data"]; !hasStepOne {
				t.Error("step 2's context file should contain step_one_data from step 1")
			}
		}
	}
}

func TestFullPipeline_RetryThenSucceed(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, nil)

	// Create a counter file to track attempts
	counterFile := filepath.Join(repoRoot, "attempt_counter")

	motifDir := filepath.Join(repoRoot, ".motif")
	testutil.WritePipeline(t, motifDir, "retry", `name: retry
steps:
  - name: flaky
    script: |
      COUNT=0
      if [ -f "`+counterFile+`" ]; then
        COUNT=$(cat "`+counterFile+`")
      fi
      COUNT=$((COUNT + 1))
      echo $COUNT > "`+counterFile+`"
      if [ $COUNT -lt 3 ]; then
        echo "Attempt $COUNT: failing" >&2
        exit 1
      fi
      echo "Attempt $COUNT: success"
    retries: 3
    retry_backoff: fixed
`)

	engine, _ := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("retry")

	runState, runErr := engine.Run(context.Background(), *p, trigger)
	if runErr != nil {
		t.Fatalf("pipeline should succeed after retries, got error: %v", runErr)
	}

	if runState.Status != state.StatusCompleted {
		t.Errorf("Status = %q, want completed", runState.Status)
	}

	// Verify counter reached 3
	counterContent, _ := os.ReadFile(counterFile)
	count := strings.TrimSpace(string(counterContent))
	if count != "3" {
		t.Errorf("attempt counter = %q, want '3'", count)
	}

	// Step result should show retries used
	if len(runState.StepResults) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(runState.StepResults))
	}
	if runState.StepResults[0].RetriesUsed != 2 {
		t.Errorf("RetriesUsed = %d, want 2", runState.StepResults[0].RetriesUsed)
	}
}

func TestFullPipeline_FallbackOnFailure(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, nil)
	fallbackMarker := filepath.Join(repoRoot, "fallback_ran")

	motifDir := filepath.Join(repoRoot, ".motif")
	testutil.WritePipeline(t, motifDir, "fallback", `name: fallback
steps:
  - name: risky
    script: exit 1
    fallback:
      script: touch `+fallbackMarker+`
`)

	engine, _ := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("fallback")

	runState, runErr := engine.Run(context.Background(), *p, trigger)

	// Pipeline should fail even though fallback ran
	if runErr == nil {
		t.Fatal("expected pipeline failure")
	}
	if runState.Status != state.StatusFailed {
		t.Errorf("Status = %q, want failed", runState.Status)
	}

	// Fallback should have run
	if _, statErr := os.Stat(fallbackMarker); os.IsNotExist(statErr) {
		t.Error("fallback should have created the marker file")
	}
}

func TestFullPipeline_TimeoutKillsScript(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, nil)

	motifDir := filepath.Join(repoRoot, ".motif")
	testutil.WritePipeline(t, motifDir, "timeout", `name: timeout
steps:
  - name: slow
    script: sleep 3600
    timeout: 1s
`)

	engine, _ := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("timeout")

	start := time.Now()
	runState, runErr := engine.Run(context.Background(), *p, trigger)
	elapsed := time.Since(start)

	if runErr == nil {
		t.Fatal("expected timeout error")
	}
	if runState.Status != state.StatusFailed {
		t.Errorf("Status = %q, want failed", runState.Status)
	}

	// Should complete quickly (not wait for sleep 3600)
	if elapsed > 10*time.Second {
		t.Errorf("timeout took %s, should be ~1s", elapsed)
	}

	// Error should mention timeout
	if !strings.Contains(runErr.Error(), "SCRIPT_TIMEOUT") {
		t.Errorf("error should mention timeout, got %q", runErr.Error())
	}
}

func TestFullPipeline_StateFilesWritten(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"stateful": `name: stateful
steps:
  - name: one
    script: echo step_one
  - name: two
    script: echo step_two
  - name: three
    script: |
      echo '{"result": "done"}'
    output_key: final
`,
	})

	engine, stateStore := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("stateful")

	runState, runErr := engine.Run(context.Background(), *p, trigger)
	if runErr != nil {
		t.Fatalf("pipeline run failed: %v", runErr)
	}

	// Verify state.json exists and is valid
	runDir := filepath.Join(motifDir, "runs", runState.RunID)
	stateContent, readErr := os.ReadFile(filepath.Join(runDir, "state.json"))
	if readErr != nil {
		t.Fatalf("failed to read state.json: %v", readErr)
	}

	var loadedState state.RunState
	if jsonErr := json.Unmarshal(stateContent, &loadedState); jsonErr != nil {
		t.Fatalf("state.json is not valid JSON: %v", jsonErr)
	}
	if loadedState.Status != state.StatusCompleted {
		t.Errorf("state.json Status = %q, want completed", loadedState.Status)
	}
	if len(loadedState.StepResults) != 3 {
		t.Errorf("state.json StepResults = %d, want 3", len(loadedState.StepResults))
	}

	// Verify log.json has 3 NDJSON lines
	logContent, logErr := os.ReadFile(filepath.Join(runDir, "log.json"))
	if logErr != nil {
		t.Fatalf("failed to read log.json: %v", logErr)
	}
	lines := strings.Split(strings.TrimSpace(string(logContent)), "\n")
	if len(lines) != 3 {
		t.Errorf("log.json should have 3 lines, got %d", len(lines))
	}

	// Each line should be valid JSON
	for i, line := range lines {
		var entry state.StepResult
		if jsonErr := json.Unmarshal([]byte(line), &entry); jsonErr != nil {
			t.Errorf("log.json line %d is not valid JSON: %v", i, jsonErr)
		}
	}

	// Verify ListRuns works
	summaries, listErr := stateStore.ListRuns()
	if listErr != nil {
		t.Fatalf("ListRuns error: %v", listErr)
	}
	if len(summaries) != 1 {
		t.Errorf("ListRuns returned %d runs, want 1", len(summaries))
	}
}

func TestFullPipeline_ValidateDetectsErrors(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"broken": `steps:
  - name: s
    script: echo hi
`,
	})

	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	_, loadErr := loader.Load("broken")
	if loadErr == nil {
		t.Fatal("expected validation error for pipeline without name")
	}
	if !strings.Contains(loadErr.Error(), "name") {
		t.Errorf("error should mention missing name, got %q", loadErr.Error())
	}
}

func TestFullPipeline_AgentStepFailsWithoutAPIKey(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, nil)

	// Create a dummy agent file so validation passes
	motifDir := filepath.Join(repoRoot, ".motif")
	agentsDir := filepath.Join(motifDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	agentPath := filepath.Join(agentsDir, "dummy.md")
	os.WriteFile(agentPath, []byte("# dummy agent"), 0o644)

	testutil.WritePipeline(t, motifDir, "mixed", `name: mixed
steps:
  - name: agent-step
    agent: ./agents/dummy.md
  - name: script-step
    script: echo "script ran"
`)

	engine, _ := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("mixed")

	// Without an API key, agent steps should fail (not skip).
	_, runErr := engine.Run(context.Background(), *p, trigger)
	if runErr == nil {
		t.Fatal("expected pipeline to fail when agent step runs without API key")
	}
	if !strings.Contains(runErr.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("error should mention ANTHROPIC_API_KEY, got: %v", runErr)
	}
}

func TestFullPipeline_IntentPassedToContext(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"intent": `name: intent
steps:
  - name: check
    script: cat "$MOTIF_CONTEXT_FILE"
`,
	})

	engine, _ := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: "add notification preferences",
	}

	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("intent")

	runState, runErr := engine.Run(context.Background(), *p, trigger)
	if runErr != nil {
		t.Fatalf("pipeline run failed: %v", runErr)
	}

	// Verify the intent made it into the context
	if runState.Context.Trigger.Intent != "add notification preferences" {
		t.Errorf("Intent = %q, want %q",
			runState.Context.Trigger.Intent, "add notification preferences")
	}

	// Verify the script's stdout contains the intent (from context file)
	stdout := runState.StepResults[0].Stdout
	if !strings.Contains(stdout, "add notification preferences") {
		t.Error("script stdout should contain the intent from context file")
	}
}

func TestFullPipeline_MultipleOutputKeys(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"multi": `name: multi
steps:
  - name: alpha
    script: |
      echo '{"from": "alpha"}'
    output_key: alpha_data
  - name: beta
    script: |
      echo '{"from": "beta"}'
    output_key: beta_data
  - name: gamma
    script: |
      echo '{"from": "gamma"}'
    output_key: gamma_data
`,
	})

	engine, _ := buildEngine(t, repoRoot)
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)
	p, _ := loader.Load("multi")

	runState, runErr := engine.Run(context.Background(), *p, trigger)
	if runErr != nil {
		t.Fatalf("pipeline run failed: %v", runErr)
	}

	// All three output keys should be in context
	for _, key := range []string{"alpha_data", "beta_data", "gamma_data"} {
		if _, ok := runState.Context.Data[key]; !ok {
			t.Errorf("context should contain %q", key)
		}
	}
}
