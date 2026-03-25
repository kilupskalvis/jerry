package pipeline

import (
	"context"
	"time"

	"github.com/kilupskalvis/jerry/internal/contextstore"
)

// StepExecutor executes a single pipeline step.
type StepExecutor interface {
	Execute(ctx context.Context, step Step, store ContextReader) (*StepOutput, error)
	CanExecute(step Step) bool
}

// StepOutput holds the output of a successful step execution.
type StepOutput struct {
	Data              any
	Stdout            string
	Stderr            string
	ExitCode          int
	Duration          time.Duration
	OutputKeyOverride string // takes precedence over step.OutputKey when set
	Iterations        int
	ToolCalls         int
	TokensInput       int
	TokensOutput      int
}

// ErrSkipStep signals that a step should be skipped, not treated as a failure.
type ErrSkipStep struct {
	Reason string
}

func (e ErrSkipStep) Error() string { return e.Reason }

// ContextReader provides read-only access to the context store.
type ContextReader interface {
	Get() contextstore.Context
	GetKeys(keys []string) map[string]any
	WriteContextFile() (path string, cleanup func(), err error)
}
