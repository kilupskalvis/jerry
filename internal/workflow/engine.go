package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/kilupskalvis/jerry/internal/contextstore"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/state"
)

const DefaultRetryBaseDelay = 2 * time.Second

// Engine orchestrates the execution of a workflow.
type Engine struct {
	executors      []StepExecutor
	stateStore     state.StateStore
	printer        *output.Printer
	defaultTimeout time.Duration

	// OnStoreCreated is called when a new context store is created,
	// allowing executors to receive a reference to the store.
	OnStoreCreated func(*contextstore.Store)
}

func NewEngine(executors []StepExecutor, stateStore state.StateStore, printer *output.Printer, defaultTimeout time.Duration) *Engine {
	return &Engine{
		executors:      executors,
		stateStore:     stateStore,
		printer:        printer,
		defaultTimeout: defaultTimeout,
	}
}

// Run executes a workflow from start to finish.
func (e *Engine) Run(ctx context.Context, workflowDef Workflow, trigger contextstore.TriggerData) (*state.RunState, error) {
	runID := generateRunID()
	startTime := time.Now()

	ctxStore := contextstore.NewStore(runID, trigger)
	if e.OnStoreCreated != nil {
		e.OnStoreCreated(ctxStore)
	}

	runState := state.RunState{
		RunID:        runID,
		WorkflowName: workflowDef.Name,
		WorkflowFile: workflowDef.SourceFile,
		Status:       state.StatusRunning,
		StartedAt:    startTime,
		TotalSteps:   len(workflowDef.Steps),
		StepResults:  []state.StepResult{},
		Context:      ctxStore.Snapshot(),
	}

	if initErr := e.stateStore.InitRun(runState); initErr != nil {
		return &runState, jerrerr.Wrap(jerrerr.CodeStateWriteFailed,
			"failed to initialize run state", initErr)
	}

	logWriter, logErr := state.NewLogWriter(e.stateStore.RunDir(runID))
	if logErr != nil {
		e.printer.Warning("failed to create log writer: %s", logErr)
	}
	defer func() { _ = logWriter.Close() }()

	stepNames := make([]string, len(workflowDef.Steps))
	for i, s := range workflowDef.Steps {
		stepNames[i] = s.Name
	}
	logWriter.Log(state.LogPipelineStart, "", state.PipelineStartData{
		RunID:         runID,
		Pipeline:      workflowDef.Name,
		TriggerIntent: trigger.Intent,
		Steps:         stepNames,
	})

	e.printer.Info("Running workflow: %s (%d steps)", workflowDef.Name, len(workflowDef.Steps))

	execErr := e.runSteps(ctx, workflowDef.Steps, 0, ctxStore, &runState, logWriter)

	if runState.Status == state.StatusRunning {
		runState.Status = state.StatusCompleted
		now := time.Now()
		runState.CompletedAt = &now
		runState.Context = ctxStore.Snapshot()
		_ = e.stateStore.SaveFinal(runState)
		e.printer.PipelineComplete(time.Since(startTime), runID, totalTokens(runState.StepResults))
	}

	e.logWorkflowEnd(logWriter, &runState, startTime)

	return &runState, execErr
}

// RunFrom resumes a workflow from a specific step index.
func (e *Engine) RunFrom(
	ctx context.Context,
	workflowDef Workflow,
	fromStep int,
	existingStore *contextstore.Store,
	existingState *state.RunState,
) (*state.RunState, error) {
	existingState.Status = state.StatusRunning

	if e.OnStoreCreated != nil {
		e.OnStoreCreated(existingStore)
	}

	logWriter, logErr := state.NewLogWriter(e.stateStore.RunDir(existingState.RunID))
	if logErr != nil {
		e.printer.Warning("failed to create log writer for resume: %s", logErr)
	}
	defer func() { _ = logWriter.Close() }()

	e.printer.Info("Resuming workflow: %s from step %q (run: %s)",
		workflowDef.Name, workflowDef.Steps[fromStep].Name, existingState.RunID)

	execErr := e.runSteps(ctx, workflowDef.Steps, fromStep, existingStore, existingState, logWriter)

	if existingState.Status == state.StatusRunning {
		existingState.Status = state.StatusCompleted
		now := time.Now()
		existingState.CompletedAt = &now
		existingState.Context = existingStore.Snapshot()
		_ = e.stateStore.SaveFinal(*existingState)
		e.printer.PipelineComplete(time.Since(existingState.StartedAt), existingState.RunID, totalTokens(existingState.StepResults))
	}

	e.logWorkflowEnd(logWriter, existingState, existingState.StartedAt)

	return existingState, execErr
}

func (e *Engine) runSteps(
	ctx context.Context,
	steps []Step,
	fromStep int,
	ctxStore *contextstore.Store,
	runState *state.RunState,
	logWriter *state.LogWriter,
) error {
	for i := fromStep; i < len(steps); i++ {
		step := steps[i]
		runState.CurrentStep = i
		stepStartTime := time.Now()

		executor := e.findExecutor(step)
		if executor == nil {
			result := state.StepResult{
				Name: step.Name, Type: stepType(step), Status: state.StepSkipped,
				StartedAt: stepStartTime, CompletedAt: time.Now(),
				Error: &state.ErrorDetail{Code: "NO_EXECUTOR", Message: "no executor found for this step"},
			}
			runState.StepResults = append(runState.StepResults, result)
			e.printer.StepSkipped(step.Name, "no executor found")
			_ = e.stateStore.SaveCheckpoint(*runState)
			continue
		}

		e.printer.StepStart(step.Name)
		logWriter.Log(state.LogStepStart, step.Name, state.StepStartData{
			Type: stepType(step), AgentFile: step.Agent,
		})

		// Build prevOutputs from cumulative context for this step.
		prevOutputs := buildPrevOutputs(ctxStore)

		stepTimeout := e.resolveTimeout(step)
		stepCtx, stepCancel := context.WithTimeout(ctx, stepTimeout)
		stepOutput, retriesUsed, execErr := e.executeWithRetries(stepCtx, step, executor, prevOutputs)
		stepCancel()

		stepDuration := time.Since(stepStartTime)

		if execErr != nil {
			result := state.StepResult{
				Name: step.Name, Type: stepType(step), Status: state.StepFailed,
				StartedAt: stepStartTime, CompletedAt: time.Now(),
				DurationMs: stepDuration.Milliseconds(), RetriesUsed: retriesUsed,
				Error: &state.ErrorDetail{Code: "STEP_FAILED", Message: execErr.Error()},
			}
			runState.StepResults = append(runState.StepResults, result)
			runState.Status = state.StatusFailed
			runState.Context = ctxStore.Snapshot()
			now := time.Now()
			runState.CompletedAt = &now
			_ = e.stateStore.SaveCheckpoint(*runState)
			_ = e.stateStore.SaveFinal(*runState)

			e.printer.StepFailed(step.Name, execErr.Error())
			logWriter.Log(state.LogStepEnd, step.Name, state.StepEndData{
				Status: "failed", DurationMs: stepDuration.Milliseconds(),
			})
			e.printer.PipelineFailed(step.Name, execErr.Error(), runState.WorkflowName, runState.RunID)

			return execErr
		}

		// Success — append output to cumulative context.
		if stepOutput != nil {
			ctxStore.Append(stepOutput.StepName, stepOutput.Data)
		}

		result := state.StepResult{
			Name: step.Name, Type: stepType(step), Status: state.StepSuccess,
			StartedAt: stepStartTime, CompletedAt: time.Now(),
			DurationMs: stepDuration.Milliseconds(), RetriesUsed: retriesUsed,
		}
		if stepOutput != nil {
			result.Stdout = stepOutput.Data
		}
		runState.StepResults = append(runState.StepResults, result)
		runState.Context = ctxStore.Snapshot()
		_ = e.stateStore.SaveCheckpoint(*runState)

		e.printer.StepSuccess(step.Name, stepDuration, 0, 0, 0, 0)
		logWriter.Log(state.LogStepEnd, step.Name, state.StepEndData{
			Status: "success", DurationMs: stepDuration.Milliseconds(),
		})
	}

	return nil
}

// buildPrevOutputs converts cumulative context store entries to StepOutput slice.
func buildPrevOutputs(ctxStore *contextstore.Store) []StepOutput {
	prev := ctxStore.PreviousOutputs()
	outputs := make([]StepOutput, len(prev))
	for i, p := range prev {
		outputs[i] = StepOutput{StepName: p.Name, Data: p.Output}
	}
	return outputs
}

func (e *Engine) executeWithRetries(ctx context.Context, step Step, exec StepExecutor, prevOutputs []StepOutput) (*StepOutput, int, error) {
	maxAttempts := step.Retries + 1
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		stepOutput, execErr := exec.Execute(ctx, step, prevOutputs)
		if execErr == nil {
			return stepOutput, attempt - 1, nil
		}
		lastErr = execErr

		if attempt < maxAttempts {
			e.printer.Warning("step %q failed (attempt %d/%d), retrying in %s...",
				step.Name, attempt, maxAttempts, DefaultRetryBaseDelay)

			timer := time.NewTimer(DefaultRetryBaseDelay)
			select {
			case <-timer.C:
				continue
			case <-ctx.Done():
				timer.Stop()
				return nil, attempt - 1, ctx.Err()
			}
		}
	}

	return nil, maxAttempts - 1, lastErr
}

func (e *Engine) logWorkflowEnd(lw *state.LogWriter, runState *state.RunState, startTime time.Time) {
	var completed, failed, skipped int
	for _, r := range runState.StepResults {
		switch r.Status {
		case state.StepSuccess:
			completed++
		case state.StepFailed:
			failed++
		case state.StepSkipped:
			skipped++
		}
	}
	lw.Log(state.LogPipelineEnd, "", state.PipelineEndData{
		Status:         string(runState.Status),
		DurationMs:     time.Since(startTime).Milliseconds(),
		StepsCompleted: completed,
		StepsFailed:    failed,
		StepsSkipped:   skipped,
	})
}

func (e *Engine) findExecutor(step Step) StepExecutor {
	for _, exec := range e.executors {
		if exec.CanExecute(step) {
			return exec
		}
	}
	return nil
}

func (e *Engine) resolveTimeout(step Step) time.Duration {
	if step.Timeout.Duration > 0 {
		return step.Timeout.Duration
	}
	return e.defaultTimeout
}

func stepType(step Step) string {
	if step.Agent != "" {
		return "agent"
	}
	return "script"
}

func totalTokens(results []state.StepResult) int {
	total := 0
	for _, r := range results {
		total += r.TokensInput + r.TokensOutput
	}
	return total
}

func generateRunID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("run_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("run_%s", hex.EncodeToString(b))
}
