package workflow

import (
	"context"
	"time"
)

// StepExecutor executes a single workflow step.
type StepExecutor interface {
	Execute(ctx context.Context, step Step, prevOutputs []StepOutput) (*StepOutput, error)
	CanExecute(step Step) bool
}

// StepOutput holds the output of a completed step.
type StepOutput struct {
	StepName string
	Data     string
	Duration time.Duration
}
