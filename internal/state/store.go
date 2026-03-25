// FileStateStore: persists run state to disk as JSON files.

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kilupskalvis/jerry/internal/errors"
)

// FileStateStore implements StateStore using the local filesystem.
type FileStateStore struct {
	runsDir string // absolute path to .jerry/runs/
}

// NewFileStateStore creates a state store that persists to the given directory.
func NewFileStateStore(runsDir string) *FileStateStore {
	return &FileStateStore{runsDir: runsDir}
}

// InitRun creates the run directory and writes the initial state file.
func (s *FileStateStore) InitRun(runState RunState) error {
	runDir := filepath.Join(s.runsDir, runState.RunID)

	if mkdirErr := os.MkdirAll(runDir, 0o755); mkdirErr != nil {
		return errors.Wrap(errors.CodeStateWriteFailed,
			fmt.Sprintf("failed to create run directory %q", runDir), mkdirErr)
	}

	statePath := filepath.Join(runDir, "state.json")
	return AtomicWriteJSON(statePath, runState)
}

// SaveCheckpoint overwrites the state file and appends the latest step result
// to the NDJSON log.
func (s *FileStateStore) SaveCheckpoint(runState RunState) error {
	runDir := filepath.Join(s.runsDir, runState.RunID)
	statePath := filepath.Join(runDir, "state.json")

	if err := AtomicWriteJSON(statePath, runState); err != nil {
		return err
	}

	// Append latest step result to log.json (NDJSON)
	if len(runState.StepResults) > 0 {
		latestResult := runState.StepResults[len(runState.StepResults)-1]
		return s.appendToLog(runDir, latestResult)
	}

	return nil
}

// SaveFinal writes the terminal state (completed or failed).
func (s *FileStateStore) SaveFinal(runState RunState) error {
	runDir := filepath.Join(s.runsDir, runState.RunID)
	statePath := filepath.Join(runDir, "state.json")
	return AtomicWriteJSON(statePath, runState)
}

// LoadRun reads a run state from disk by run ID.
func (s *FileStateStore) LoadRun(runID string) (*RunState, error) {
	statePath := filepath.Join(s.runsDir, runID, "state.json")

	content, readErr := os.ReadFile(statePath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil, errors.New(errors.CodePipelineNotFound,
				fmt.Sprintf("run %q not found in %s", runID, s.runsDir))
		}
		return nil, errors.Wrap(errors.CodeStateWriteFailed,
			fmt.Sprintf("failed to read state for run %q", runID), readErr)
	}

	var loaded RunState
	if unmarshalErr := json.Unmarshal(content, &loaded); unmarshalErr != nil {
		return nil, errors.Wrap(errors.CodeStateWriteFailed,
			fmt.Sprintf("failed to parse state.json for run %q", runID), unmarshalErr)
	}

	return &loaded, nil
}

// ListRuns returns summary information for all stored runs,
// ordered by start time (most recent first).
func (s *FileStateStore) ListRuns() ([]RunSummary, error) {
	entries, readErr := os.ReadDir(s.runsDir)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return []RunSummary{}, nil
		}
		return nil, errors.Wrap(errors.CodeStateWriteFailed,
			"failed to read runs directory", readErr)
	}

	var summaries []RunSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		loaded, loadErr := s.LoadRun(entry.Name())
		if loadErr != nil {
			continue // skip corrupt/unreadable runs
		}

		summaries = append(summaries, RunSummary{
			RunID:        loaded.RunID,
			PipelineName: loaded.PipelineName,
			Status:       loaded.Status,
			StartedAt:    loaded.StartedAt,
			StepCount:    loaded.TotalSteps,
		})
	}

	// Sort by start time, most recent first
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].StartedAt.After(summaries[j].StartedAt)
	})

	return summaries, nil
}

// RunDir returns the absolute path to a run's directory.
func (s *FileStateStore) RunDir(runID string) string {
	return filepath.Join(s.runsDir, runID)
}

// appendToLog appends a step result as a single NDJSON line to log.json.
func (s *FileStateStore) appendToLog(runDir string, result StepResult) error {
	logPath := filepath.Join(runDir, "log.json")

	line, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to marshal step result for log", marshalErr)
	}
	line = append(line, '\n')

	logFile, openErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if openErr != nil {
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to open log file", openErr)
	}
	defer func() { _ = logFile.Close() }()

	if _, writeErr := logFile.Write(line); writeErr != nil {
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to append to log file", writeErr)
	}

	return nil
}
