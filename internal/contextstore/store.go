// Package contextstore manages the shared context object that flows
// through a Motif pipeline. The context is the single communication
// channel between steps — steps do not communicate directly.
//
// All read operations return deep copies to prevent external mutations
// of internal state. Thread-safe via read-write mutex.
package contextstore

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/kilupskalvis/motif/internal/errors"
)

// ProtocolVersion is the current version of the Motif context protocol.
const ProtocolVersion = "1.0"

// Context represents the shared state flowing through the pipeline.
type Context struct {
	ProtocolVersion string         `json:"protocol_version"`
	RunID           string         `json:"run_id"`
	Trigger         TriggerData    `json:"trigger"`
	Data            map[string]any `json:"data"`
}

// TriggerData holds information about what initiated the pipeline run.
type TriggerData struct {
	Type       string         `json:"type"`
	Source     string         `json:"source"`
	Intent     string         `json:"intent,omitempty"`
	RawPayload map[string]any `json:"raw_payload,omitempty"`
}

// reservedKeys are context keys that cannot be written by steps.
var reservedKeys = map[string]bool{
	"protocol_version": true,
	"run_id":           true,
	"trigger":          true,
}

// Store manages the context object in memory during pipeline execution.
type Store struct {
	pipelineContext Context
	mu              sync.RWMutex
}

// NewStore creates a new context store with the given initial state.
func NewStore(runID string, trigger TriggerData) *Store {
	return &Store{
		pipelineContext: Context{
			ProtocolVersion: ProtocolVersion,
			RunID:           runID,
			Trigger:         trigger,
			Data:            make(map[string]any),
		},
	}
}

// Get returns a deep copy of the full context object.
func (s *Store) Get() Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deepCopy()
}

// GetKeys returns a deep copy of only the specified data keys.
// Keys that don't exist in the context are omitted from the result.
func (s *Store) GetKeys(keys []string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]any)
	for _, key := range keys {
		if val, ok := s.pipelineContext.Data[key]; ok {
			result[key] = deepCopyValue(val)
		}
	}
	return result
}

// Set writes a value to the specified data key.
// Returns an error if the key is reserved (protocol_version, run_id, trigger).
// Performs full replacement — if the key already exists, the old value is
// entirely replaced (not merged).
func (s *Store) Set(key string, value any) error {
	if reservedKeys[key] {
		return errors.New(errors.CodeContextWriteDenied,
			fmt.Sprintf("cannot write to reserved context key %q", key))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pipelineContext.Data[key] = deepCopyValue(value)
	return nil
}

// Snapshot returns a full deep copy of the context for persistence.
func (s *Store) Snapshot() Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deepCopy()
}

// WriteContextFile writes the context as formatted JSON to a temporary
// file and returns the file path and a cleanup function that removes the file.
func (s *Store) WriteContextFile() (path string, cleanup func(), err error) {
	snapshot := s.Snapshot()

	data, marshalErr := json.MarshalIndent(snapshot, "", "  ")
	if marshalErr != nil {
		return "", nil, errors.Wrap(errors.CodeStateWriteFailed,
			"failed to marshal context to JSON", marshalErr)
	}

	tmpFile, createErr := os.CreateTemp("", "motif-context-*.json")
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

// deepCopy creates a full deep copy of the internal context via JSON
// marshal/unmarshal. Simple, correct, adequate performance for
// context-sized data.
func (s *Store) deepCopy() Context {
	data, _ := json.Marshal(s.pipelineContext)
	var copied Context
	_ = json.Unmarshal(data, &copied)
	if copied.Data == nil {
		copied.Data = make(map[string]any)
	}
	return copied
}

// deepCopyValue creates a deep copy of an arbitrary value via JSON roundtrip.
func deepCopyValue(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return value // fallback: return as-is if unmarshalable
	}
	var copied any
	_ = json.Unmarshal(data, &copied)
	return copied
}
