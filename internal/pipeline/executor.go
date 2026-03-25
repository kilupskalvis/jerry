// Step executor interface and shared types for pipeline step execution.

package pipeline

import (
	"context"
	"time"

	"github.com/kilupskalvis/jerry/internal/contextstore"
)

// StepExecutor executes a single pipeline step and returns the result.
// The pipeline engine uses this interface to dispatch steps — it does
// not know whether a step is a script, an agent, or something else.
//
// Implementations:
//   - script.Executor (Phase 1)
//   - agent.Executor  (Phase 2)
type StepExecutor interface {
	// Execute runs the step with the given context.
	// The context.Context parameter carries cancellation and timeout.
	// The ContextReader provides read access to previous step outputs.
	// Returns a StepOutput on success or an error on failure.
	Execute(stepCtx context.Context, step Step, store ContextReader) (*StepOutput, error)

	// CanExecute reports whether this executor handles the given step.
	// The engine iterates executors and delegates to the first that
	// returns true.
	CanExecute(step Step) bool
}

// StepOutput holds the output of a successful step execution.
type StepOutput struct {
	// Data is the structured output to merge into the context.
	// Nil if the step produces no context output.
	Data any

	// Stdout is the raw standard output from the step.
	Stdout string

	// Stderr is the raw standard error from the step.
	Stderr string

	// ExitCode is the process exit code (for script steps).
	// Zero for agent steps that complete successfully.
	ExitCode int

	// Duration is how long the step took to execute.
	Duration time.Duration

	// OutputKeyOverride, if non-empty, takes precedence over step.OutputKey
	// from the pipeline YAML. Used by agent steps where the output_key is
	// defined in the agent frontmatter (the authoritative source for agents).
	OutputKeyOverride string
}

// ErrSkipStep is a sentinel error returned by executors to indicate
// that a step should be skipped (not treated as a failure). The engine
// logs the reason as a warning and continues to the next step.
type ErrSkipStep struct {
	Reason string
}

// Error implements the error interface.
func (e ErrSkipStep) Error() string { return e.Reason }

// ContextReader provides read-only access to the context store.
// Passed to executors so they can provide context to steps without
// allowing mutation.
type ContextReader interface {
	// Get returns the full context object (deep copy).
	Get() contextstore.Context

	// GetKeys returns only the specified data keys (deep copy).
	GetKeys(keys []string) map[string]any

	// WriteContextFile writes the context to a temporary file and
	// returns the file path. The caller is responsible for cleanup
	// via the returned function.
	WriteContextFile() (path string, cleanup func(), err error)
}
