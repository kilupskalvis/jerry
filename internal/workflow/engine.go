package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

const (
	DefaultRetryBaseDelay = 2 * time.Second
	MaxRetryDelay         = 30 * time.Second
)

// Engine orchestrates the execution of a workflow.
type Engine struct {
	executors      []StepExecutor
	stateStore     run.StateStore
	printer        *output.Printer
	defaultTimeout time.Duration

	// OnStoreCreated is called when a new context store is created,
	// allowing executors to receive a reference to the store.
	OnStoreCreated func(*run.ContextStore)
}

func NewEngine(executors []StepExecutor, stateStore run.StateStore, printer *output.Printer, defaultTimeout time.Duration) *Engine {
	return &Engine{
		executors:      executors,
		stateStore:     stateStore,
		printer:        printer,
		defaultTimeout: defaultTimeout,
	}
}

// Run executes a workflow from start to finish.
func (e *Engine) Run(ctx context.Context, workflowDef Workflow, triggerData trigger.TriggerData) (*run.RunState, error) {
	runID := generateRunID()
	startTime := time.Now()

	ctxStore := run.NewContextStore(runID, triggerData)
	if e.OnStoreCreated != nil {
		e.OnStoreCreated(ctxStore)
	}

	runState := run.RunState{
		RunID:        runID,
		WorkflowName: workflowDef.Name,
		WorkflowFile: workflowDef.SourceFile,
		Status:       run.StatusRunning,
		StartedAt:    startTime,
		TotalSteps:   len(workflowDef.Steps),
		StepResults:  []run.StepResult{},
		Context:      ctxStore.Snapshot(),
	}

	if initErr := e.stateStore.InitRun(runState); initErr != nil {
		return &runState, jerrerr.Wrap(jerrerr.CodeStateWriteFailed,
			"failed to initialize run state", initErr)
	}

	logWriter, logErr := run.NewLogWriter(e.stateStore.RunDir(runID))
	if logErr != nil {
		e.printer.Warning("failed to create log writer: %s", logErr)
	}
	defer func() { _ = logWriter.Close() }()

	stepNames := make([]string, len(workflowDef.Steps))
	for i, s := range workflowDef.Steps {
		stepNames[i] = s.Name
	}
	logWriter.Log(run.LogWorkflowStart, "", run.WorkflowStartData{
		RunID:         runID,
		Workflow:      workflowDef.Name,
		TriggerIntent: triggerData.Intent,
		Steps:         stepNames,
	})

	e.printer.Info("Running workflow: %s (%d steps)", workflowDef.Name, len(workflowDef.Steps))

	execErr := e.runSteps(ctx, workflowDef.Steps, 0, ctxStore, &runState, logWriter)

	if runState.Status == run.StatusRunning {
		runState.Status = run.StatusCompleted
		now := time.Now()
		runState.CompletedAt = &now
		runState.Context = ctxStore.Snapshot()
		_ = e.stateStore.SaveFinal(runState)
		e.printer.WorkflowComplete(time.Since(startTime), runID, totalTokens(runState.StepResults))
	}

	e.logWorkflowEnd(logWriter, &runState, startTime)

	return &runState, execErr
}

// RunFrom resumes a workflow from a specific step index.
func (e *Engine) RunFrom(
	ctx context.Context,
	workflowDef Workflow,
	fromStep int,
	existingStore *run.ContextStore,
	existingState *run.RunState,
) (*run.RunState, error) {
	existingState.Status = run.StatusRunning

	if e.OnStoreCreated != nil {
		e.OnStoreCreated(existingStore)
	}

	logWriter, logErr := run.NewLogWriter(e.stateStore.RunDir(existingState.RunID))
	if logErr != nil {
		e.printer.Warning("failed to create log writer for resume: %s", logErr)
	}
	defer func() { _ = logWriter.Close() }()

	e.printer.Info("Resuming workflow: %s from step %q (run: %s)",
		workflowDef.Name, workflowDef.Steps[fromStep].Name, existingState.RunID)

	execErr := e.runSteps(ctx, workflowDef.Steps, fromStep, existingStore, existingState, logWriter)

	if existingState.Status == run.StatusRunning {
		existingState.Status = run.StatusCompleted
		now := time.Now()
		existingState.CompletedAt = &now
		existingState.Context = existingStore.Snapshot()
		_ = e.stateStore.SaveFinal(*existingState)
		e.printer.WorkflowComplete(time.Since(existingState.StartedAt), existingState.RunID, totalTokens(existingState.StepResults))
	}

	e.logWorkflowEnd(logWriter, existingState, existingState.StartedAt)

	return existingState, execErr
}

func (e *Engine) runSteps(
	ctx context.Context,
	steps []Step,
	fromStep int,
	ctxStore *run.ContextStore,
	runState *run.RunState,
	logWriter *run.LogWriter,
) error {
	for i := fromStep; i < len(steps); i++ {
		step := steps[i]
		runState.CurrentStep = i
		stepStartTime := time.Now()

		executor := e.findExecutor(step)
		if executor == nil {
			result := run.StepResult{
				Name: step.Name, Type: stepType(step), Status: run.StepSkipped,
				StartedAt: stepStartTime, CompletedAt: time.Now(),
				Error: &run.ErrorDetail{Code: "NO_EXECUTOR", Message: "no executor found for this step"},
			}
			runState.StepResults = append(runState.StepResults, result)
			e.printer.StepSkipped(step.Name, "no executor found")
			_ = e.stateStore.SaveCheckpoint(*runState)
			continue
		}

		e.printer.StepStart(step.Name)
		logWriter.Log(run.LogStepStart, step.Name, run.StepStartData{
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
			result := run.StepResult{
				Name: step.Name, Type: stepType(step), Status: run.StepFailed,
				StartedAt: stepStartTime, CompletedAt: time.Now(),
				DurationMs: stepDuration.Milliseconds(), RetriesUsed: retriesUsed,
				Error: &run.ErrorDetail{Code: "STEP_FAILED", Message: execErr.Error()},
			}
			runState.StepResults = append(runState.StepResults, result)
			runState.Status = run.StatusFailed
			runState.Context = ctxStore.Snapshot()
			now := time.Now()
			runState.CompletedAt = &now
			_ = e.stateStore.SaveCheckpoint(*runState)
			_ = e.stateStore.SaveFinal(*runState)

			e.printer.StepFailed(step.Name, execErr.Error())
			logWriter.Log(run.LogStepEnd, step.Name, run.StepEndData{
				Status: "failed", DurationMs: stepDuration.Milliseconds(),
			})
			e.printer.WorkflowFailed(step.Name, execErr.Error())

			return execErr
		}

		// Success — append output to cumulative context.
		if stepOutput != nil {
			ctxStore.Append(stepOutput.StepName, stepOutput.Data)
		}

		result := run.StepResult{
			Name: step.Name, Type: stepType(step), Status: run.StepSuccess,
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
		logWriter.Log(run.LogStepEnd, step.Name, run.StepEndData{
			Status: "success", DurationMs: stepDuration.Milliseconds(),
		})
	}

	return nil
}

// buildPrevOutputs converts cumulative context store entries to StepOutput slice.
func buildPrevOutputs(ctxStore *run.ContextStore) []StepOutput {
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
			delay := DefaultRetryBaseDelay * (1 << (attempt - 1))
			if delay > MaxRetryDelay {
				delay = MaxRetryDelay
			}
			e.printer.Warning("step %q failed (attempt %d/%d), retrying in %s...",
				step.Name, attempt, maxAttempts, delay)

			timer := time.NewTimer(delay)
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

func (e *Engine) logWorkflowEnd(lw *run.LogWriter, runState *run.RunState, startTime time.Time) {
	var completed, failed, skipped int
	for _, r := range runState.StepResults {
		switch r.Status {
		case run.StepSuccess:
			completed++
		case run.StepFailed:
			failed++
		case run.StepSkipped:
			skipped++
		}
	}
	lw.Log(run.LogWorkflowEnd, "", run.WorkflowEndData{
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

func totalTokens(results []run.StepResult) int {
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
