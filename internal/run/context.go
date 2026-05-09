package run

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

const ProtocolVersion = "1.0"

// ContextEntry holds the output of a completed step for context passing.
type ContextEntry struct {
	Name   string `json:"name"`
	Output string `json:"output"`
}

// Context represents the cumulative state flowing through the workflow.
type Context struct {
	ProtocolVersion string              `json:"protocol_version"`
	RunID           string              `json:"run_id"`
	Trigger         trigger.TriggerData `json:"trigger"`
	Steps           []ContextEntry      `json:"steps"`
}

// ContextStore manages the context object in memory during workflow execution.
type ContextStore struct {
	ctx Context
	mu  sync.RWMutex
}

// NewContextStore creates a new context store with the given trigger.
func NewContextStore(runID string, triggerData trigger.TriggerData) *ContextStore {
	return &ContextStore{
		ctx: Context{
			ProtocolVersion: ProtocolVersion,
			RunID:           runID,
			Trigger:         triggerData,
		},
	}
}

// Append adds a step result to the cumulative context.
func (s *ContextStore) Append(name, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.Steps = append(s.ctx.Steps, ContextEntry{Name: name, Output: output})
}

// PreviousOutputs returns a copy of all step results so far.
func (s *ContextStore) PreviousOutputs() []ContextEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ContextEntry, len(s.ctx.Steps))
	copy(result, s.ctx.Steps)
	return result
}

// Snapshot returns a deep copy of the context for persistence.
func (s *ContextStore) Snapshot() Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, _ := json.Marshal(s.ctx)
	var copied Context
	_ = json.Unmarshal(data, &copied)
	return copied
}

// RestoreFromSnapshot creates a ContextStore from a previously saved Context.
func RestoreFromSnapshot(snapshot Context) *ContextStore {
	return &ContextStore{ctx: snapshot}
}

// Trigger returns the trigger data.
func (s *ContextStore) Trigger() trigger.TriggerData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ctx.Trigger
}

// WriteContextFile writes the context as JSON to a temporary file.
func (s *ContextStore) WriteContextFile() (path string, cleanup func(), err error) {
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
