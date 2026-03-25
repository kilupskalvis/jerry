package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/contextstore"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/pipeline"
	"github.com/kilupskalvis/jerry/internal/state"
)

// mockExecutor is a configurable StepExecutor for testing.
type mockExecutor struct {
	canHandle   func(step pipeline.Step) bool
	executeFunc func(stepCtx context.Context, step pipeline.Step, store pipeline.ContextReader) (*pipeline.StepOutput, error)
	calls       []string // step names executed
}

func (m *mockExecutor) CanExecute(step pipeline.Step) bool {
	if m.canHandle != nil {
		return m.canHandle(step)
	}
	return step.Script != ""
}

func (m *mockExecutor) Execute(stepCtx context.Context, step pipeline.Step, store pipeline.ContextReader) (*pipeline.StepOutput, error) {
	m.calls = append(m.calls, step.Name)
	if m.executeFunc != nil {
		return m.executeFunc(stepCtx, step, store)
	}
	return &pipeline.StepOutput{Duration: time.Millisecond}, nil
}

// mockStateStore records calls without touching the filesystem.
type mockStateStore struct {
	initCalls       int
	checkpointCalls int
	finalCalls      int
	lastState       *state.RunState
	runsDir         string // temp dir for log files
}

func (m *mockStateStore) InitRun(runState state.RunState) error {
	m.initCalls++
	m.lastState = &runState
	return nil
}

func (m *mockStateStore) SaveCheckpoint(runState state.RunState) error {
	m.checkpointCalls++
	m.lastState = &runState
	return nil
}

func (m *mockStateStore) SaveFinal(runState state.RunState) error {
	m.finalCalls++
	m.lastState = &runState
	return nil
}

func (m *mockStateStore) LoadRun(_ string) (*state.RunState, error) {
	return m.lastState, nil
}

func (m *mockStateStore) ListRuns() ([]state.RunSummary, error) {
	return nil, nil
}

func (m *mockStateStore) RunDir(runID string) string {
	base := m.runsDir
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, runID)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func newTestEngine(exec pipeline.StepExecutor, stateStore state.StateStore) *pipeline.Engine {
	printer := output.NewPrinter(devNull{}, devNull{})
	return pipeline.NewEngine(
		[]pipeline.StepExecutor{exec},
		stateStore,
		printer,
		10*time.Minute,
	)
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }

func TestRun_SequentialExecution(t *testing.T) {
	exec := &mockExecutor{}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name: "test",
		Steps: []pipeline.Step{
			{Name: "one", Script: "echo 1"},
			{Name: "two", Script: "echo 2"},
			{Name: "three", Script: "echo 3"},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, p, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(exec.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d: %v", len(exec.calls), exec.calls)
	}
	if exec.calls[0] != "one" || exec.calls[1] != "two" || exec.calls[2] != "three" {
		t.Errorf("steps executed out of order: %v", exec.calls)
	}
	if runState.Status != state.StatusCompleted {
		t.Errorf("Status = %q, want %q", runState.Status, state.StatusCompleted)
	}
}

func TestRun_ContextFlowBetweenSteps(t *testing.T) {
	callCount := 0
	exec := &mockExecutor{
		executeFunc: func(stepCtx context.Context, step pipeline.Step, store pipeline.ContextReader) (*pipeline.StepOutput, error) {
			callCount++
			if callCount == 1 {
				// First step produces output
				return &pipeline.StepOutput{
					Data:     map[string]any{"produced": "data"},
					Duration: time.Millisecond,
				}, nil
			}
			// Second step reads context
			snapshot := store.Get()
			if _, ok := snapshot.Data["result"]; !ok {
				t.Error("step 2 should see step 1's output in context")
			}
			return &pipeline.StepOutput{Duration: time.Millisecond}, nil
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name: "test",
		Steps: []pipeline.Step{
			{Name: "one", Script: "echo 1", OutputKey: "result"},
			{Name: "two", Script: "echo 2"},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	_, err := engine.Run(ctx, p, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_AgentStepSkipped(t *testing.T) {
	scriptExec := &mockExecutor{}
	agentExec := &mockExecutor{
		canHandle: func(step pipeline.Step) bool { return step.Agent != "" },
		executeFunc: func(_ context.Context, step pipeline.Step, _ pipeline.ContextReader) (*pipeline.StepOutput, error) {
			return nil, pipeline.ErrSkipStep{Reason: "agent steps require Phase 2"}
		},
	}
	store := &mockStateStore{}
	printer := output.NewPrinter(devNull{}, devNull{})
	engine := pipeline.NewEngine(
		[]pipeline.StepExecutor{agentExec, scriptExec},
		store,
		printer,
		10*time.Minute,
	)

	p := pipeline.Pipeline{
		Name: "test",
		Steps: []pipeline.Step{
			{Name: "gen", Agent: "./agents/gen.md"},
			{Name: "validate", Script: "echo ok"},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, p, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Agent step should be skipped, script step should run
	if len(scriptExec.calls) != 1 {
		t.Errorf("script executor should have been called once, got %d", len(scriptExec.calls))
	}
	if runState.Status != state.StatusCompleted {
		t.Errorf("pipeline should complete despite skipped step")
	}

	// Check step results
	skippedFound := false
	for _, sr := range runState.StepResults {
		if sr.Name == "gen" && sr.Status == state.StepSkipped {
			skippedFound = true
		}
	}
	if !skippedFound {
		t.Error("expected a skipped step result for 'gen'")
	}
}

func TestRun_RetrySuccess(t *testing.T) {
	attempts := 0
	exec := &mockExecutor{
		executeFunc: func(_ context.Context, _ pipeline.Step, _ pipeline.ContextReader) (*pipeline.StepOutput, error) {
			attempts++
			if attempts <= 2 {
				return nil, jerrerr.New(jerrerr.CodeScriptFailed, "failed")
			}
			return &pipeline.StepOutput{Duration: time.Millisecond}, nil
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name: "test",
		Steps: []pipeline.Step{
			{Name: "flaky", Script: "flaky-cmd", Retries: 2, RetryBackoffStrategy: "fixed"},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, p, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runState.Status != state.StatusCompleted {
		t.Errorf("Status = %q, want completed (should succeed on 3rd attempt)", runState.Status)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRun_RetryExhausted(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(_ context.Context, _ pipeline.Step, _ pipeline.ContextReader) (*pipeline.StepOutput, error) {
			return nil, jerrerr.New(jerrerr.CodeScriptFailed, "always fails")
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name: "test",
		Steps: []pipeline.Step{
			{Name: "broken", Script: "exit 1", Retries: 3},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, p, trigger)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}

	if runState.Status != state.StatusFailed {
		t.Errorf("Status = %q, want failed", runState.Status)
	}
}

func TestRun_FallbackExecuted(t *testing.T) {
	callCount := 0
	fallbackCalled := false
	exec := &mockExecutor{
		executeFunc: func(_ context.Context, step pipeline.Step, _ pipeline.ContextReader) (*pipeline.StepOutput, error) {
			callCount++
			if step.Name == "risky" {
				return nil, jerrerr.New(jerrerr.CodeScriptFailed, "failed")
			}
			// This is the fallback execution
			fallbackCalled = true
			return &pipeline.StepOutput{Duration: time.Millisecond}, nil
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name: "test",
		Steps: []pipeline.Step{
			{Name: "risky", Script: "exit 1", Fallback: &pipeline.FallbackDef{Script: "echo fallback"}},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, p, trigger)

	// Pipeline still fails even if fallback succeeds
	if err == nil {
		t.Fatal("expected error even with fallback")
	}
	if !fallbackCalled {
		t.Error("fallback should have been called")
	}
	if runState.Status != state.StatusFailed {
		t.Errorf("Status = %q, want failed", runState.Status)
	}
}

func TestRun_StepTimeout(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(stepCtx context.Context, _ pipeline.Step, _ pipeline.ContextReader) (*pipeline.StepOutput, error) {
			<-stepCtx.Done()
			return nil, jerrerr.New(jerrerr.CodeScriptTimeout, "timed out")
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name: "test",
		Steps: []pipeline.Step{
			{Name: "slow", Script: "sleep 60", Timeout: pipeline.Duration{Duration: 100 * time.Millisecond}},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	_, err := engine.Run(ctx, p, trigger)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRun_StateCheckpoints(t *testing.T) {
	exec := &mockExecutor{}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name: "test",
		Steps: []pipeline.Step{
			{Name: "one", Script: "echo 1"},
			{Name: "two", Script: "echo 2"},
			{Name: "three", Script: "echo 3"},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	_, err := engine.Run(ctx, p, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.initCalls != 1 {
		t.Errorf("InitRun calls = %d, want 1", store.initCalls)
	}
	// One checkpoint per step
	if store.checkpointCalls != 3 {
		t.Errorf("SaveCheckpoint calls = %d, want 3", store.checkpointCalls)
	}
	if store.finalCalls != 1 {
		t.Errorf("SaveFinal calls = %d, want 1", store.finalCalls)
	}
}

func TestRun_GenerateRunID(t *testing.T) {
	exec := &mockExecutor{}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name:  "test",
		Steps: []pipeline.Step{{Name: "s", Script: "echo hi"}},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, p, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should match run_ + 16 hex chars
	pattern := `^run_[0-9a-f]{16}$`
	matched, _ := regexp.MatchString(pattern, runState.RunID)
	if !matched {
		t.Errorf("RunID %q does not match pattern %q", runState.RunID, pattern)
	}
}

func TestRun_NoExecutorFound(t *testing.T) {
	// Executor that handles nothing
	exec := &mockExecutor{
		canHandle: func(_ pipeline.Step) bool { return false },
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	p := pipeline.Pipeline{
		Name:  "test",
		Steps: []pipeline.Step{{Name: "orphan", Script: "echo hi"}},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, p, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step should be skipped
	if len(runState.StepResults) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(runState.StepResults))
	}
	if runState.StepResults[0].Status != state.StepSkipped {
		t.Errorf("Status = %q, want skipped", runState.StepResults[0].Status)
	}
}
