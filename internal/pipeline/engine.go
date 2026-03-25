package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/kilupskalvis/jerry/internal/contextstore"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/state"
)

const (
	DefaultRetryBaseDelay = 2 * time.Second
	MaxRetryDelay         = 60 * time.Second
)

// Engine orchestrates the execution of a pipeline.
type Engine struct {
	executors      []StepExecutor
	stateStore     state.StateStore
	printer        *output.Printer
	defaultTimeout time.Duration
}

// NewEngine creates a pipeline engine with the given executors and state store.
func NewEngine(executors []StepExecutor, stateStore state.StateStore, printer *output.Printer, defaultTimeout time.Duration) *Engine {
	return &Engine{
		executors:      executors,
		stateStore:     stateStore,
		printer:        printer,
		defaultTimeout: defaultTimeout,
	}
}

// Run executes a pipeline from start to finish.
func (e *Engine) Run(ctx context.Context, pipelineDef Pipeline, trigger contextstore.TriggerData) (*state.RunState, error) {
	runID := generateRunID()
	startTime := time.Now()

	ctxStore := contextstore.NewStore(runID, trigger)

	runState := state.RunState{
		RunID:        runID,
		PipelineName: pipelineDef.Name,
		PipelineFile: pipelineDef.SourceFile,
		Status:       state.StatusRunning,
		StartedAt:    startTime,
		TotalSteps:   len(pipelineDef.Steps),
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

	stepNames := make([]string, len(pipelineDef.Steps))
	for i, s := range pipelineDef.Steps {
		stepNames[i] = s.Name
	}
	logWriter.Log(state.LogPipelineStart, "", state.PipelineStartData{
		RunID:         runID,
		Pipeline:      pipelineDef.Name,
		TriggerIntent: trigger.Intent,
		Steps:         stepNames,
	})

	ctx = state.WithLogWriter(ctx, logWriter)

	e.printer.Info("Running pipeline: %s (%d steps)", pipelineDef.Name, len(pipelineDef.Steps))

	execErr := e.runSteps(ctx, pipelineDef.Steps, 0, ctxStore, &runState, logWriter)

	if runState.Status == state.StatusRunning {
		runState.Status = state.StatusCompleted
		now := time.Now()
		runState.CompletedAt = &now
		runState.Context = ctxStore.Snapshot()
		_ = e.stateStore.SaveFinal(runState)
		e.printer.PipelineComplete(time.Since(startTime), runID)
	}

	e.logPipelineEnd(logWriter, &runState, startTime)

	return &runState, execErr
}

// RunFrom resumes a pipeline from a specific step index, using an existing
// context store and run state.
func (e *Engine) RunFrom(
	ctx context.Context,
	pipelineDef Pipeline,
	fromStep int,
	existingStore *contextstore.Store,
	existingState *state.RunState,
) (*state.RunState, error) {
	existingState.Status = state.StatusRunning

	logWriter, logErr := state.NewLogWriter(e.stateStore.RunDir(existingState.RunID))
	if logErr != nil {
		e.printer.Warning("failed to create log writer for resume: %s", logErr)
	}
	defer func() { _ = logWriter.Close() }()

	ctx = state.WithLogWriter(ctx, logWriter)

	e.printer.Info("Resuming pipeline: %s from step %q (run: %s)",
		pipelineDef.Name, pipelineDef.Steps[fromStep].Name, existingState.RunID)

	execErr := e.runSteps(ctx, pipelineDef.Steps, fromStep, existingStore, existingState, logWriter)

	if existingState.Status == state.StatusRunning {
		existingState.Status = state.StatusCompleted
		now := time.Now()
		existingState.CompletedAt = &now
		existingState.Context = existingStore.Snapshot()
		_ = e.stateStore.SaveFinal(*existingState)
		e.printer.PipelineComplete(time.Since(existingState.StartedAt), existingState.RunID)
	}

	e.logPipelineEnd(logWriter, existingState, existingState.StartedAt)

	return existingState, execErr
}

// runSteps executes pipeline steps from the given index. Shared by Run and RunFrom.
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

		stepTimeout := e.resolveTimeout(step)
		stepCtx, stepCancel := context.WithTimeout(ctx, stepTimeout)
		stepCtx = state.WithStepName(stepCtx, step.Name)
		stepOutput, retriesUsed, execErr := e.executeWithRetries(stepCtx, step, executor, ctxStore)
		stepCancel()

		stepDuration := time.Since(stepStartTime)

		var skipErr ErrSkipStep
		if errors.As(execErr, &skipErr) {
			result := state.StepResult{
				Name: step.Name, Type: stepType(step), Status: state.StepSkipped,
				StartedAt: stepStartTime, CompletedAt: time.Now(), DurationMs: stepDuration.Milliseconds(),
			}
			runState.StepResults = append(runState.StepResults, result)
			e.printer.StepSkipped(step.Name, skipErr.Reason)
			runState.Context = ctxStore.Snapshot()
			_ = e.stateStore.SaveCheckpoint(*runState)
			continue
		}

		if execErr != nil {
			if step.Fallback != nil {
				e.executeFallback(ctx, step, ctxStore)
			}

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
			e.printer.PipelineFailed(step.Name, execErr.Error(), runState.RunID)

			return execErr
		}

		// Success — merge output into context.
		if stepOutput != nil && stepOutput.Data != nil {
			outputKey := step.OutputKey
			if stepOutput.OutputKeyOverride != "" {
				outputKey = stepOutput.OutputKeyOverride
			}
			if outputKey != "" {
				_ = ctxStore.Set(outputKey, stepOutput.Data)
			}
		}

		result := state.StepResult{
			Name: step.Name, Type: stepType(step), Status: state.StepSuccess,
			StartedAt: stepStartTime, CompletedAt: time.Now(),
			DurationMs: stepDuration.Milliseconds(), RetriesUsed: retriesUsed,
		}
		if stepOutput != nil {
			result.Stdout = stepOutput.Stdout
			result.Stderr = stepOutput.Stderr
		}
		runState.StepResults = append(runState.StepResults, result)
		runState.Context = ctxStore.Snapshot()
		_ = e.stateStore.SaveCheckpoint(*runState)

		var iterations, toolCalls, tokensIn, tokensOut int
		if stepOutput != nil {
			iterations = stepOutput.Iterations
			toolCalls = stepOutput.ToolCalls
			tokensIn = stepOutput.TokensInput
			tokensOut = stepOutput.TokensOutput
		}
		e.printer.StepSuccess(step.Name, stepDuration, iterations, toolCalls, tokensIn, tokensOut)
		logWriter.Log(state.LogStepEnd, step.Name, state.StepEndData{
			Status: "success", DurationMs: stepDuration.Milliseconds(),
		})
	}

	return nil
}

func (e *Engine) logPipelineEnd(lw *state.LogWriter, runState *state.RunState, startTime time.Time) {
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

func (e *Engine) executeWithRetries(ctx context.Context, step Step, exec StepExecutor, store ContextReader) (*StepOutput, int, error) {
	maxAttempts := step.Retries + 1
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		stepOutput, execErr := exec.Execute(ctx, step, store)
		if execErr == nil {
			return stepOutput, attempt - 1, nil
		}
		lastErr = execErr

		var skipErr ErrSkipStep
		if errors.As(execErr, &skipErr) {
			return nil, 0, execErr
		}

		if attempt < maxAttempts {
			waitDuration := backoffDuration(attempt, step.RetryBackoffStrategy)
			e.printer.Warning("step %q failed (attempt %d/%d), retrying in %s...",
				step.Name, attempt, maxAttempts, waitDuration)

			timer := time.NewTimer(waitDuration)
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

func (e *Engine) executeFallback(ctx context.Context, step Step, store ContextReader) {
	if step.Fallback == nil {
		return
	}

	fallbackStep := Step{
		Name:   step.Name + "_fallback",
		Script: step.Fallback.Script,
		Agent:  step.Fallback.Agent,
	}

	executor := e.findExecutor(fallbackStep)
	if executor == nil {
		e.printer.Warning("no executor found for fallback of step %q", step.Name)
		return
	}

	fallbackCtx, fallbackCancel := context.WithTimeout(ctx, e.defaultTimeout)
	defer fallbackCancel()

	lw := state.LogWriterFrom(ctx)
	fallbackStart := time.Now()
	_, fallbackErr := executor.Execute(fallbackCtx, fallbackStep, store)

	status := "success"
	errMsg := ""
	if fallbackErr != nil {
		status = "failed"
		errMsg = fallbackErr.Error()
		e.printer.Warning("fallback for step %q failed: %s", step.Name, errMsg)
	}

	if lw != nil {
		lw.Log(state.LogStepEnd, step.Name+"_fallback", state.StepEndData{
			Status:     status,
			DurationMs: time.Since(fallbackStart).Milliseconds(),
		})
	}

	if errMsg == "" {
		e.printer.Info("  fallback for step %q succeeded", step.Name)
	}
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

func backoffDuration(attempt int, strategy string) time.Duration {
	switch strategy {
	case "exponential":
		delay := DefaultRetryBaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		if delay > MaxRetryDelay {
			return MaxRetryDelay
		}
		return delay
	default:
		return DefaultRetryBaseDelay
	}
}

func stepType(step Step) string {
	if step.Agent != "" {
		return "agent"
	}
	return "script"
}

func generateRunID() string {
	b := make([]byte, 8)
	_, readErr := rand.Read(b)
	if readErr != nil {
		return fmt.Sprintf("run_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("run_%s", hex.EncodeToString(b))
}
