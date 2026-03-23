package agent

import (
	"context"
	"fmt"

	"github.com/kilupskalvis/motif/internal/pipeline"
)

// Compile-time interface compliance assertion.
var _ pipeline.StepExecutor = (*Executor)(nil)

// Executor is the agent step executor. In Phase 1, it returns a skip
// sentinel so the pipeline engine logs a warning and continues to the
// next step. Phase 2 replaces this with the full agentic loop.
type Executor struct{}

// NewExecutor creates an agent executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// CanExecute returns true if the step has an Agent field set.
func (e *Executor) CanExecute(step pipeline.Step) bool {
	return step.Agent != ""
}

// Execute returns ErrSkipStep to indicate agent steps are not yet supported.
// The pipeline engine treats this sentinel as a skip (not a failure).
func (e *Executor) Execute(_ context.Context, step pipeline.Step, _ pipeline.ContextReader) (*pipeline.StepOutput, error) {
	return nil, pipeline.ErrSkipStep{
		Reason: fmt.Sprintf("agent steps require Phase 2 (step %q)", step.Name),
	}
}
