package workflow_test

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
	"github.com/kilupskalvis/jerry/internal/state"
	"github.com/kilupskalvis/jerry/internal/workflow"
)

type mockExecutor struct {
	canHandle   func(step workflow.Step) bool
	executeFunc func(ctx context.Context, step workflow.Step, prevOutputs []workflow.StepOutput) (*workflow.StepOutput, error)
	calls       []string
}

func (m *mockExecutor) CanExecute(step workflow.Step) bool {
	if m.canHandle != nil {
		return m.canHandle(step)
	}
	return step.Run != ""
}

func (m *mockExecutor) Execute(ctx context.Context, step workflow.Step, prevOutputs []workflow.StepOutput) (*workflow.StepOutput, error) {
	m.calls = append(m.calls, step.Name)
	if m.executeFunc != nil {
		return m.executeFunc(ctx, step, prevOutputs)
	}
	return &workflow.StepOutput{StepName: step.Name, Data: "ok", Duration: time.Millisecond}, nil
}

type mockStateStore struct {
	initCalls       int
	checkpointCalls int
	finalCalls      int
	lastState       *state.RunState
	runsDir         string
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

func newTestEngine(exec workflow.StepExecutor, stateStore state.StateStore) *workflow.Engine {
	printer := output.NewPrinter(devNull{}, devNull{})
	return workflow.NewEngine(
		[]workflow.StepExecutor{exec},
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

	w := workflow.Workflow{
		Name: "test",
		Steps: []workflow.Step{
			{Name: "one", Run: "echo 1"},
			{Name: "two", Run: "echo 2"},
			{Name: "three", Run: "echo 3"},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, w, trigger)
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

func TestRun_CumulativeContextFlow(t *testing.T) {
	callCount := 0
	exec := &mockExecutor{
		executeFunc: func(_ context.Context, step workflow.Step, prevOutputs []workflow.StepOutput) (*workflow.StepOutput, error) {
			callCount++
			if callCount == 1 {
				return &workflow.StepOutput{
					StepName: step.Name,
					Data:     `{"produced": "data"}`,
					Duration: time.Millisecond,
				}, nil
			}
			if len(prevOutputs) != 1 {
				t.Errorf("step 2 should see 1 previous output, got %d", len(prevOutputs))
			}
			if prevOutputs[0].StepName != "one" {
				t.Errorf("previous output name = %q, want %q", prevOutputs[0].StepName, "one")
			}
			return &workflow.StepOutput{StepName: step.Name, Data: "ok", Duration: time.Millisecond}, nil
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	w := workflow.Workflow{
		Name: "test",
		Steps: []workflow.Step{
			{Name: "one", Run: "echo 1"},
			{Name: "two", Run: "echo 2"},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	_, err := engine.Run(ctx, w, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_RetrySuccess(t *testing.T) {
	attempts := 0
	exec := &mockExecutor{
		executeFunc: func(_ context.Context, step workflow.Step, _ []workflow.StepOutput) (*workflow.StepOutput, error) {
			attempts++
			if attempts <= 2 {
				return nil, jerrerr.New(jerrerr.CodeScriptFailed, "failed")
			}
			return &workflow.StepOutput{StepName: step.Name, Data: "ok", Duration: time.Millisecond}, nil
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	w := workflow.Workflow{
		Name: "test",
		Steps: []workflow.Step{
			{Name: "flaky", Run: "flaky-cmd", Retries: 2},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, w, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runState.Status != state.StatusCompleted {
		t.Errorf("Status = %q, want completed", runState.Status)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRun_RetryExhausted(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(_ context.Context, _ workflow.Step, _ []workflow.StepOutput) (*workflow.StepOutput, error) {
			return nil, jerrerr.New(jerrerr.CodeScriptFailed, "always fails")
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	w := workflow.Workflow{
		Name: "test",
		Steps: []workflow.Step{
			{Name: "broken", Run: "exit 1", Retries: 3},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, w, trigger)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}

	if runState.Status != state.StatusFailed {
		t.Errorf("Status = %q, want failed", runState.Status)
	}
}

func TestRun_StepTimeout(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(ctx context.Context, _ workflow.Step, _ []workflow.StepOutput) (*workflow.StepOutput, error) {
			<-ctx.Done()
			return nil, jerrerr.New(jerrerr.CodeScriptTimeout, "timed out")
		},
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	w := workflow.Workflow{
		Name: "test",
		Steps: []workflow.Step{
			{Name: "slow", Run: "sleep 60", Timeout: workflow.Duration{Duration: 100 * time.Millisecond}},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	_, err := engine.Run(ctx, w, trigger)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRun_StateCheckpoints(t *testing.T) {
	exec := &mockExecutor{}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	w := workflow.Workflow{
		Name: "test",
		Steps: []workflow.Step{
			{Name: "one", Run: "echo 1"},
			{Name: "two", Run: "echo 2"},
			{Name: "three", Run: "echo 3"},
		},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	_, err := engine.Run(ctx, w, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.initCalls != 1 {
		t.Errorf("InitRun calls = %d, want 1", store.initCalls)
	}
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

	w := workflow.Workflow{
		Name:  "test",
		Steps: []workflow.Step{{Name: "s", Run: "echo hi"}},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, w, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pattern := `^run_[0-9a-f]{16}$`
	matched, _ := regexp.MatchString(pattern, runState.RunID)
	if !matched {
		t.Errorf("RunID %q does not match pattern %q", runState.RunID, pattern)
	}
}

func TestRun_NoExecutorFound(t *testing.T) {
	exec := &mockExecutor{
		canHandle: func(_ workflow.Step) bool { return false },
	}
	store := &mockStateStore{}
	engine := newTestEngine(exec, store)

	w := workflow.Workflow{
		Name:  "test",
		Steps: []workflow.Step{{Name: "orphan", Run: "echo hi"}},
	}

	ctx := context.Background()
	trigger := contextstore.TriggerData{Type: "manual", Source: "cli"}
	runState, err := engine.Run(ctx, w, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runState.StepResults) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(runState.StepResults))
	}
	if runState.StepResults[0].Status != state.StepSkipped {
		t.Errorf("Status = %q, want skipped", runState.StepResults[0].Status)
	}
}
