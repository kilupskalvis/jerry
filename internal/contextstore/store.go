// Package contextstore manages the shared context that flows through a workflow.
package contextstore

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/kilupskalvis/jerry/internal/errors"
)

const ProtocolVersion = "1.0"

// TriggerData holds information about what initiated the workflow run.
type TriggerData struct {
	Type       string         `json:"type"`
	Source     string         `json:"source"`
	Intent     string         `json:"intent,omitempty"`
	RawPayload map[string]any `json:"raw_payload,omitempty"`
}

// StepResult holds the output of a completed step for context passing.
type StepResult struct {
	Name   string `json:"name"`
	Output string `json:"output"`
}

// Context represents the cumulative state flowing through the workflow.
type Context struct {
	ProtocolVersion string       `json:"protocol_version"`
	RunID           string       `json:"run_id"`
	Trigger         TriggerData  `json:"trigger"`
	Steps           []StepResult `json:"steps"`
}

// Store manages the context object in memory during workflow execution.
type Store struct {
	ctx Context
	mu  sync.RWMutex
}

// NewStore creates a new context store with the given trigger.
func NewStore(runID string, trigger TriggerData) *Store {
	return &Store{
		ctx: Context{
			ProtocolVersion: ProtocolVersion,
			RunID:           runID,
			Trigger:         trigger,
		},
	}
}

// Append adds a step result to the cumulative context.
func (s *Store) Append(name, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.Steps = append(s.ctx.Steps, StepResult{Name: name, Output: output})
}

// PreviousOutputs returns a copy of all step results so far.
func (s *Store) PreviousOutputs() []StepResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]StepResult, len(s.ctx.Steps))
	copy(result, s.ctx.Steps)
	return result
}

// Snapshot returns a deep copy of the context for persistence.
func (s *Store) Snapshot() Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, _ := json.Marshal(s.ctx)
	var copied Context
	_ = json.Unmarshal(data, &copied)
	return copied
}

// RestoreFromSnapshot creates a Store from a previously saved Context.
func RestoreFromSnapshot(snapshot Context) *Store {
	return &Store{ctx: snapshot}
}

// Trigger returns the trigger data.
func (s *Store) Trigger() TriggerData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ctx.Trigger
}

// WriteContextFile writes the context as JSON to a temporary file.
func (s *Store) WriteContextFile() (path string, cleanup func(), err error) {
	snapshot := s.Snapshot()

	data, marshalErr := json.MarshalIndent(snapshot, "", "  ")
	if marshalErr != nil {
		return "", nil, errors.Wrap(errors.CodeStateWriteFailed,
			"failed to marshal context to JSON", marshalErr)
	}

	tmpFile, createErr := os.CreateTemp("", "jerry-context-*.json")
	if createErr != nil {
		return "", nil, errors.Wrap(errors.CodeStateWriteFailed,
			"failed to create temp context file", createErr)
	}

	if _, writeErr := tmpFile.Write(data); writeErr != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", nil, errors.Wrap(errors.CodeStateWriteFailed,
			"failed to write context to temp file", writeErr)
	}

	if closeErr := tmpFile.Close(); closeErr != nil {
		_ = os.Remove(tmpFile.Name())
		return "", nil, errors.Wrap(errors.CodeStateWriteFailed,
			"failed to close temp context file", closeErr)
	}

	filePath := tmpFile.Name()
	return filePath, func() { _ = os.Remove(filePath) }, nil
}
