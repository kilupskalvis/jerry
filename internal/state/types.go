// Package state manages persistence of pipeline run state to disk.
// After each step, state is saved as a checkpoint enabling resumability.
package state

import (
	"time"

	"github.com/kilupskalvis/motif/internal/contextstore"
)

// RunStatus represents the current state of a pipeline run.
type RunStatus string

const (
	StatusRunning   RunStatus = "running"
	StatusCompleted RunStatus = "completed"
	StatusFailed    RunStatus = "failed"
)

// StepStatus represents the outcome of a step.
type StepStatus string

const (
	StepSuccess StepStatus = "success"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
)

// RunState holds the persistent state of a pipeline run.
type RunState struct {
	RunID        string              `json:"run_id"`
	PipelineName string              `json:"pipeline_name"`
	PipelineFile string              `json:"pipeline_file"`
	Status       RunStatus           `json:"status"`
	StartedAt    time.Time           `json:"started_at"`
	CompletedAt  *time.Time          `json:"completed_at,omitempty"`
	CurrentStep  int                 `json:"current_step"`
	TotalSteps   int                 `json:"total_steps"`
	StepResults  []StepResult        `json:"step_results"`
	Context      contextstore.Context `json:"context"`
}

// StepResult records the outcome of a single step execution.
type StepResult struct {
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Status      StepStatus `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt time.Time  `json:"completed_at"`
	DurationMs  int64      `json:"duration_ms"`
	RetriesUsed int        `json:"retries_used"`
	Output      any        `json:"output,omitempty"`
	Stdout      string     `json:"stdout,omitempty"`
	Stderr      string     `json:"stderr,omitempty"`
	Error       *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail holds error information for a failed step.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RunSummary is a lightweight view of a run for listing purposes.
type RunSummary struct {
	RunID        string    `json:"run_id"`
	PipelineName string    `json:"pipeline_name"`
	Status       RunStatus `json:"status"`
	StartedAt    time.Time `json:"started_at"`
	StepCount    int       `json:"step_count"`
}

// StateStore persists and retrieves pipeline run state.
// Defined as an interface for testability and future storage backends.
type StateStore interface {
	// InitRun creates the run directory and writes the initial state.
	InitRun(runState RunState) error

	// SaveCheckpoint overwrites the state file after a step completes.
	SaveCheckpoint(runState RunState) error

	// SaveFinal writes the terminal state (completed or failed).
	SaveFinal(runState RunState) error

	// LoadRun reads a run state from disk by run ID.
	LoadRun(runID string) (*RunState, error)

	// ListRuns returns summary information for all stored runs,
	// ordered by start time (most recent first).
	ListRuns() ([]RunSummary, error)
}
