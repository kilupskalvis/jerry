// Pipeline engine: sequential step orchestration with retry, fallback, and checkpointing.

package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/kilupskalvis/motif/internal/contextstore"
	motifErrors "github.com/kilupskalvis/motif/internal/errors"
	"github.com/kilupskalvis/motif/internal/output"
	"github.com/kilupskalvis/motif/internal/state"
)

// Retry backoff constants.
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
// Executors are tried in order — the first executor where CanExecute returns
// true handles the step.
func NewEngine(executors []StepExecutor, stateStore state.StateStore, printer *output.Printer, defaultTimeout time.Duration) *Engine {
	return &Engine{
		executors:      executors,
		stateStore:     stateStore,
		printer:        printer,
		defaultTimeout: defaultTimeout,
	}
}

// Run executes a pipeline from start to finish.
func (e *Engine) Run(runCtx context.Context, pipelineDef Pipeline, trigger contextstore.TriggerData) (*state.RunState, error) {
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
		return &runState, motifErrors.Wrap(motifErrors.CodeStateWriteFailed,
			"failed to initialize run state", initErr)
	}

	// Create structured log writer for this run.
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

	// Inject log writer into context for executors to use.
	runCtx = state.WithLogWriter(runCtx, logWriter)

	e.printer.Info("Running pipeline: %s (%d steps)", pipelineDef.Name, len(pipelineDef.Steps))

	// Execute each step
	for i, step := range pipelineDef.Steps {
		runState.CurrentStep = i
		stepStartTime := time.Now()

		// Find executor
		executor := e.findExecutor(step)
		if executor == nil {
			// No executor found — skip the step
			result := state.StepResult{
				Name:        step.Name,
				Type:        stepType(step),
				Status:      state.StepSkipped,
				StartedAt:   stepStartTime,
				CompletedAt: time.Now(),
				DurationMs:  0,
				Error:       &state.ErrorDetail{Code: "NO_EXECUTOR", Message: "no executor found for this step"},
			}
			runState.StepResults = append(runState.StepResults, result)
			e.printer.StepSkipped(step.Name, "no executor found")
			_ = e.stateStore.SaveCheckpoint(runState)
			continue
		}

		e.printer.StepStart(step.Name)
		logWriter.Log(state.LogStepStart, step.Name, state.StepStartData{
			Type:      stepType(step),
			AgentFile: step.Agent,
		})

		// Create step context with timeout
		stepTimeout := e.resolveTimeout(step)
		stepCtx, stepCancel := context.WithTimeout(runCtx, stepTimeout)

		// Execute with retries
		stepOutput, retriesUsed, execErr := e.executeWithRetries(stepCtx, step, executor, ctxStore)
		stepCancel()

		stepDuration := time.Since(stepStartTime)

		// Handle ErrSkipStep
		var skipErr ErrSkipStep
		if errors.As(execErr, &skipErr) {
			result := state.StepResult{
				Name:        step.Name,
				Type:        stepType(step),
				Status:      state.StepSkipped,
				StartedAt:   stepStartTime,
				CompletedAt: time.Now(),
				DurationMs:  stepDuration.Milliseconds(),
			}
			runState.StepResults = append(runState.StepResults, result)
			e.printer.StepSkipped(step.Name, skipErr.Reason)
			runState.Context = ctxStore.Snapshot()
			_ = e.stateStore.SaveCheckpoint(runState)
			continue
		}

		// Handle failure
		if execErr != nil {
			// Try fallback if configured
			if step.Fallback != nil {
				e.executeFallback(runCtx, step, ctxStore)
			}

			result := state.StepResult{
				Name:        step.Name,
				Type:        stepType(step),
				Status:      state.StepFailed,
				StartedAt:   stepStartTime,
				CompletedAt: time.Now(),
				DurationMs:  stepDuration.Milliseconds(),
				RetriesUsed: retriesUsed,
				Error:       &state.ErrorDetail{Code: "STEP_FAILED", Message: execErr.Error()},
			}
			runState.StepResults = append(runState.StepResults, result)
			runState.Status = state.StatusFailed
			runState.Context = ctxStore.Snapshot()
			now := time.Now()
			runState.CompletedAt = &now
			_ = e.stateStore.SaveCheckpoint(runState)
			_ = e.stateStore.SaveFinal(runState)

			e.printer.StepFailed(step.Name, execErr.Error())
			logWriter.Log(state.LogStepEnd, step.Name, state.StepEndData{
				Status: "failed", DurationMs: stepDuration.Milliseconds(),
			})
			e.printer.PipelineFailed(step.Name, execErr.Error(), runID)
			e.logPipelineEnd(logWriter, &runState, startTime)

			return &runState, execErr
		}

		// Success — merge output into context
		if stepOutput != nil && stepOutput.Data != nil {
			outputKey := resolveOutputKey(step, stepOutput)
			if outputKey != "" {
				_ = ctxStore.Set(outputKey, stepOutput.Data)
			}
		}

		result := state.StepResult{
			Name:        step.Name,
			Type:        stepType(step),
			Status:      state.StepSuccess,
			StartedAt:   stepStartTime,
			CompletedAt: time.Now(),
			DurationMs:  stepDuration.Milliseconds(),
			RetriesUsed: retriesUsed,
		}
		if stepOutput != nil {
			result.Stdout = stepOutput.Stdout
			result.Stderr = stepOutput.Stderr
		}
		runState.StepResults = append(runState.StepResults, result)
		runState.Context = ctxStore.Snapshot()
		_ = e.stateStore.SaveCheckpoint(runState)

		e.printer.StepSuccess(step.Name, stepDuration)
		logWriter.Log(state.LogStepEnd, step.Name, state.StepEndData{
			Status: "success", DurationMs: stepDuration.Milliseconds(),
		})
	}

	// Pipeline completed successfully
	runState.Status = state.StatusCompleted
	now := time.Now()
	runState.CompletedAt = &now
	runState.Context = ctxStore.Snapshot()
	_ = e.stateStore.SaveFinal(runState)

	e.printer.PipelineComplete(time.Since(startTime), runID)
	e.logPipelineEnd(logWriter, &runState, startTime)

	return &runState, nil
}

// logPipelineEnd writes the pipeline_end event to the log.
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

// executeWithRetries runs a step, retrying on failure according to the step's retry config.
// Returns the output, number of retries used, and error.
func (e *Engine) executeWithRetries(stepCtx context.Context, step Step, exec StepExecutor, store ContextReader) (*StepOutput, int, error) {
	maxAttempts := step.Retries + 1 // retries=0 means 1 attempt
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		stepOutput, execErr := exec.Execute(stepCtx, step, store)
		if execErr == nil {
			return stepOutput, attempt - 1, nil
		}
		lastErr = execErr

		// Don't retry ErrSkipStep
		var skipErr ErrSkipStep
		if errors.As(execErr, &skipErr) {
			return nil, 0, execErr
		}

		if attempt < maxAttempts {
			waitDuration := backoffDuration(attempt, step.RetryBackoff)
			e.printer.Warning("step %q failed (attempt %d/%d), retrying in %s...",
				step.Name, attempt, maxAttempts, waitDuration)

			select {
			case <-time.After(waitDuration):
				continue
			case <-stepCtx.Done():
				return nil, attempt - 1, stepCtx.Err()
			}
		}
	}

	return nil, maxAttempts - 1, lastErr
}

// executeFallback runs a step's fallback configuration.
func (e *Engine) executeFallback(runCtx context.Context, step Step, store ContextReader) {
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

	fallbackCtx, fallbackCancel := context.WithTimeout(runCtx, e.defaultTimeout)
	defer fallbackCancel()

	_, fallbackErr := executor.Execute(fallbackCtx, fallbackStep, store)
	if fallbackErr != nil {
		e.printer.Warning("fallback for step %q failed: %s", step.Name, fallbackErr.Error())
	}
}

// findExecutor returns the first executor that can handle the given step.
func (e *Engine) findExecutor(step Step) StepExecutor {
	for _, exec := range e.executors {
		if exec.CanExecute(step) {
			return exec
		}
	}
	return nil
}

// resolveTimeout determines the timeout for a step.
func (e *Engine) resolveTimeout(step Step) time.Duration {
	if step.Timeout.Duration > 0 {
		return step.Timeout.Duration
	}
	return e.defaultTimeout
}

// resolveOutputKey determines which context key to write the output to.
// Agent steps can override via OutputKeyOverride.
func resolveOutputKey(step Step, stepOutput *StepOutput) string {
	if stepOutput.OutputKeyOverride != "" {
		return stepOutput.OutputKeyOverride
	}
	return step.OutputKey
}

// backoffDuration calculates the wait time between retries.
func backoffDuration(attempt int, strategy string) time.Duration {
	switch strategy {
	case "exponential":
		delay := DefaultRetryBaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		if delay > MaxRetryDelay {
			return MaxRetryDelay
		}
		return delay
	default: // "fixed" or unspecified
		return DefaultRetryBaseDelay
	}
}

// stepType returns a string identifying the type of step ("script" or "agent").
func stepType(step Step) string {
	if step.Agent != "" {
		return "agent"
	}
	return "script"
}

// RunFrom resumes a pipeline from a specific step index, using an existing
// context store and run state. Used by `motif resume`.
func (e *Engine) RunFrom(
	runCtx context.Context,
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

	runCtx = state.WithLogWriter(runCtx, logWriter)

	e.printer.Info("Resuming pipeline: %s from step %q (run: %s)",
		pipelineDef.Name, pipelineDef.Steps[fromStep].Name, existingState.RunID)

	for i := fromStep; i < len(pipelineDef.Steps); i++ {
		step := pipelineDef.Steps[i]
		existingState.CurrentStep = i
		stepStartTime := time.Now()

		executor := e.findExecutor(step)
		if executor == nil {
			result := state.StepResult{
				Name: step.Name, Type: stepType(step), Status: state.StepSkipped,
				StartedAt: stepStartTime, CompletedAt: time.Now(),
				Error: &state.ErrorDetail{Code: "NO_EXECUTOR", Message: "no executor found"},
			}
			existingState.StepResults = append(existingState.StepResults, result)
			e.printer.StepSkipped(step.Name, "no executor found")
			_ = e.stateStore.SaveCheckpoint(*existingState)
			continue
		}

		e.printer.StepStart(step.Name)

		stepTimeout := e.resolveTimeout(step)
		stepCtx, stepCancel := context.WithTimeout(runCtx, stepTimeout)
		stepOutput, retriesUsed, execErr := e.executeWithRetries(stepCtx, step, executor, existingStore)
		stepCancel()

		stepDuration := time.Since(stepStartTime)

		var skipErr ErrSkipStep
		if errors.As(execErr, &skipErr) {
			result := state.StepResult{
				Name: step.Name, Type: stepType(step), Status: state.StepSkipped,
				StartedAt: stepStartTime, CompletedAt: time.Now(), DurationMs: stepDuration.Milliseconds(),
			}
			existingState.StepResults = append(existingState.StepResults, result)
			e.printer.StepSkipped(step.Name, skipErr.Reason)
			existingState.Context = existingStore.Snapshot()
			_ = e.stateStore.SaveCheckpoint(*existingState)
			continue
		}

		if execErr != nil {
			if step.Fallback != nil {
				e.executeFallback(runCtx, step, existingStore)
			}

			result := state.StepResult{
				Name: step.Name, Type: stepType(step), Status: state.StepFailed,
				StartedAt: stepStartTime, CompletedAt: time.Now(),
				DurationMs: stepDuration.Milliseconds(), RetriesUsed: retriesUsed,
				Error: &state.ErrorDetail{Code: "STEP_FAILED", Message: execErr.Error()},
			}
			existingState.StepResults = append(existingState.StepResults, result)
			existingState.Status = state.StatusFailed
			existingState.Context = existingStore.Snapshot()
			now := time.Now()
			existingState.CompletedAt = &now
			_ = e.stateStore.SaveCheckpoint(*existingState)
			_ = e.stateStore.SaveFinal(*existingState)

			e.printer.StepFailed(step.Name, execErr.Error())
			e.printer.PipelineFailed(step.Name, execErr.Error(), existingState.RunID)
			return existingState, execErr
		}

		if stepOutput != nil && stepOutput.Data != nil {
			outputKey := resolveOutputKey(step, stepOutput)
			if outputKey != "" {
				_ = existingStore.Set(outputKey, stepOutput.Data)
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
		existingState.StepResults = append(existingState.StepResults, result)
		existingState.Context = existingStore.Snapshot()
		_ = e.stateStore.SaveCheckpoint(*existingState)
		e.printer.StepSuccess(step.Name, stepDuration)
	}

	existingState.Status = state.StatusCompleted
	now := time.Now()
	existingState.CompletedAt = &now
	existingState.Context = existingStore.Snapshot()
	_ = e.stateStore.SaveFinal(*existingState)
	e.printer.PipelineComplete(time.Since(existingState.StartedAt), existingState.RunID)

	return existingState, nil
}

// generateRunID produces a unique run identifier.
// Format: "run_" + 16 hex characters (8 random bytes).
func generateRunID() string {
	b := make([]byte, 8)
	_, readErr := rand.Read(b)
	if readErr != nil {
		// Extremely unlikely fallback
		return fmt.Sprintf("run_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("run_%s", hex.EncodeToString(b))
}
