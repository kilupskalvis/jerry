package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/contextstore"
	"github.com/kilupskalvis/jerry/internal/state"
)

func newTestRunState(runID string) state.RunState {
	now := time.Now().Truncate(time.Millisecond) // truncate for JSON roundtrip
	return state.RunState{
		RunID:        runID,
		PipelineName: "test-pipeline",
		PipelineFile: ".jerry/pipelines/test.yaml",
		Status:       state.StatusRunning,
		StartedAt:    now,
		CurrentStep:  0,
		TotalSteps:   3,
		StepResults:  []state.StepResult{},
		Context: contextstore.Context{
			ProtocolVersion: "1.0",
			RunID:           runID,
			Trigger: contextstore.TriggerData{
				Type:   "manual",
				Source: "cli",
				Intent: "test",
			},
			Data: map[string]any{},
		},
	}
}

func newStepResult(name string, status state.StepStatus) state.StepResult {
	now := time.Now().Truncate(time.Millisecond)
	return state.StepResult{
		Name:        name,
		Type:        "script",
		Status:      status,
		StartedAt:   now,
		CompletedAt: now.Add(time.Second),
		DurationMs:  1000,
	}
}

func TestFileStore_InitRun(t *testing.T) {
	runsDir := t.TempDir()
	store := state.NewFileStateStore(runsDir)
	runState := newTestRunState("run_init_test")

	if err := store.InitRun(runState); err != nil {
		t.Fatalf("InitRun error: %v", err)
	}

	// Run directory should exist
	runDir := filepath.Join(runsDir, "run_init_test")
	info, err := os.Stat(runDir)
	if err != nil {
		t.Fatalf("run directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("run path is not a directory")
	}

	// state.json should exist and contain valid JSON
	stateFile := filepath.Join(runDir, "state.json")
	content, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("state.json not created: %v", err)
	}

	var loaded state.RunState
	if err := json.Unmarshal(content, &loaded); err != nil {
		t.Fatalf("state.json is not valid JSON: %v", err)
	}
	if loaded.RunID != "run_init_test" {
		t.Errorf("RunID = %q, want %q", loaded.RunID, "run_init_test")
	}
}

func TestFileStore_SaveCheckpoint(t *testing.T) {
	runsDir := t.TempDir()
	store := state.NewFileStateStore(runsDir)
	runState := newTestRunState("run_checkpoint")

	if err := store.InitRun(runState); err != nil {
		t.Fatalf("InitRun error: %v", err)
	}

	// Add a step result and save checkpoint
	runState.StepResults = append(runState.StepResults, newStepResult("step-one", state.StepSuccess))
	runState.CurrentStep = 1

	if err := store.SaveCheckpoint(runState); err != nil {
		t.Fatalf("SaveCheckpoint error: %v", err)
	}

	// state.json should reflect the update
	stateFile := filepath.Join(runsDir, "run_checkpoint", "state.json")
	content, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state.json: %v", err)
	}
	var loaded state.RunState
	if err := json.Unmarshal(content, &loaded); err != nil {
		t.Fatalf("state.json is not valid JSON: %v", err)
	}
	if loaded.CurrentStep != 1 {
		t.Errorf("CurrentStep = %d, want 1", loaded.CurrentStep)
	}
	if len(loaded.StepResults) != 1 {
		t.Fatalf("StepResults length = %d, want 1", len(loaded.StepResults))
	}

	// log.json should have an NDJSON line
	logFile := filepath.Join(runsDir, "run_checkpoint", "log.json")
	logContent, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log.json: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logContent)), "\n")
	if len(lines) != 1 {
		t.Fatalf("log.json should have 1 line, got %d", len(lines))
	}

	var logEntry state.StepResult
	if err := json.Unmarshal([]byte(lines[0]), &logEntry); err != nil {
		t.Fatalf("log.json line is not valid JSON: %v", err)
	}
	if logEntry.Name != "step-one" {
		t.Errorf("log entry name = %q, want %q", logEntry.Name, "step-one")
	}
}

func TestFileStore_SaveFinal(t *testing.T) {
	runsDir := t.TempDir()
	store := state.NewFileStateStore(runsDir)
	runState := newTestRunState("run_final")

	if err := store.InitRun(runState); err != nil {
		t.Fatalf("InitRun error: %v", err)
	}

	now := time.Now().Truncate(time.Millisecond)
	runState.Status = state.StatusCompleted
	runState.CompletedAt = &now

	if err := store.SaveFinal(runState); err != nil {
		t.Fatalf("SaveFinal error: %v", err)
	}

	stateFile := filepath.Join(runsDir, "run_final", "state.json")
	content, _ := os.ReadFile(stateFile)
	var loaded state.RunState
	_ = json.Unmarshal(content, &loaded)

	if loaded.Status != state.StatusCompleted {
		t.Errorf("Status = %q, want %q", loaded.Status, state.StatusCompleted)
	}
	if loaded.CompletedAt == nil {
		t.Fatal("CompletedAt should be set")
	}
}

func TestFileStore_LoadRun(t *testing.T) {
	runsDir := t.TempDir()
	store := state.NewFileStateStore(runsDir)
	original := newTestRunState("run_load")

	if err := store.InitRun(original); err != nil {
		t.Fatalf("InitRun error: %v", err)
	}

	loaded, err := store.LoadRun("run_load")
	if err != nil {
		t.Fatalf("LoadRun error: %v", err)
	}

	if loaded.RunID != original.RunID {
		t.Errorf("RunID = %q, want %q", loaded.RunID, original.RunID)
	}
	if loaded.PipelineName != original.PipelineName {
		t.Errorf("PipelineName = %q, want %q", loaded.PipelineName, original.PipelineName)
	}
	if loaded.Status != original.Status {
		t.Errorf("Status = %q, want %q", loaded.Status, original.Status)
	}
}

func TestFileStore_LoadRun_NotFound(t *testing.T) {
	runsDir := t.TempDir()
	store := state.NewFileStateStore(runsDir)

	_, err := store.LoadRun("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestFileStore_ListRuns(t *testing.T) {
	runsDir := t.TempDir()
	store := state.NewFileStateStore(runsDir)

	// Create runs with different timestamps
	for i, id := range []string{"run_a", "run_b", "run_c"} {
		rs := newTestRunState(id)
		rs.StartedAt = time.Now().Add(time.Duration(i) * time.Hour)
		if err := store.InitRun(rs); err != nil {
			t.Fatalf("InitRun error for %s: %v", id, err)
		}
	}

	summaries, err := store.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("ListRuns returned %d runs, want 3", len(summaries))
	}

	// Should be sorted most recent first
	if summaries[0].RunID != "run_c" {
		t.Errorf("first run should be run_c (most recent), got %q", summaries[0].RunID)
	}
}

func TestFileStore_ListRuns_Empty(t *testing.T) {
	runsDir := t.TempDir()
	store := state.NewFileStateStore(runsDir)

	summaries, err := store.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}

	if len(summaries) != 0 {
		t.Errorf("ListRuns should return empty slice, got %d", len(summaries))
	}
}

func TestAtomicWriteJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := map[string]string{"key": "value"}

	if err := state.AtomicWriteJSON(path, data); err != nil {
		t.Fatalf("AtomicWriteJSON error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var loaded map[string]string
	if err := json.Unmarshal(content, &loaded); err != nil {
		t.Fatalf("file is not valid JSON: %v", err)
	}
	if loaded["key"] != "value" {
		t.Errorf("key = %q, want %q", loaded["key"], "value")
	}

	// No .tmp file should remain
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error(".tmp file should not exist after successful write")
	}
}

func TestAtomicWriteJSON_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write first version
	if err := state.AtomicWriteJSON(path, map[string]int{"v": 1}); err != nil {
		t.Fatalf("first write error: %v", err)
	}

	// Overwrite with second version
	if err := state.AtomicWriteJSON(path, map[string]int{"v": 2}); err != nil {
		t.Fatalf("second write error: %v", err)
	}

	content, _ := os.ReadFile(path)
	var loaded map[string]int
	_ = json.Unmarshal(content, &loaded)

	if loaded["v"] != 2 {
		t.Errorf("v = %d, want 2 (should be overwritten)", loaded["v"])
	}
}
