package run_test

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

func newTestContextStore() *run.ContextStore {
	return run.NewContextStore("run_test123", trigger.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: "test intent",
	})
}

func TestNewContextStore(t *testing.T) {
	store := newTestContextStore()
	snapshot := store.Snapshot()

	if snapshot.ProtocolVersion != "1.0" {
		t.Errorf("ProtocolVersion = %q, want %q", snapshot.ProtocolVersion, "1.0")
	}
	if snapshot.RunID != "run_test123" {
		t.Errorf("RunID = %q, want %q", snapshot.RunID, "run_test123")
	}
	if snapshot.Trigger.Type != "manual" {
		t.Errorf("Trigger.Type = %q, want %q", snapshot.Trigger.Type, "manual")
	}
	if snapshot.Trigger.Intent != "test intent" {
		t.Errorf("Trigger.Intent = %q, want %q", snapshot.Trigger.Intent, "test intent")
	}
	if len(snapshot.Steps) != 0 {
		t.Errorf("Steps should be empty, got %d", len(snapshot.Steps))
	}
}

func TestContextStore_Append(t *testing.T) {
	store := newTestContextStore()

	store.Append("plan", `{"summary": "build health endpoint"}`)
	store.Append("generate", `{"artifacts": ["main.go"]}`)

	outputs := store.PreviousOutputs()
	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(outputs))
	}
	if outputs[0].Name != "plan" {
		t.Errorf("first output name = %q, want %q", outputs[0].Name, "plan")
	}
	if outputs[1].Name != "generate" {
		t.Errorf("second output name = %q, want %q", outputs[1].Name, "generate")
	}
}

func TestContextStore_PreviousOutputs_Empty(t *testing.T) {
	store := newTestContextStore()
	outputs := store.PreviousOutputs()
	if len(outputs) != 0 {
		t.Errorf("expected 0 outputs, got %d", len(outputs))
	}
}

func TestContextStore_PreviousOutputs_IsCopy(t *testing.T) {
	store := newTestContextStore()
	store.Append("plan", "output")

	outputs := store.PreviousOutputs()
	outputs[0].Name = "mutated"

	original := store.PreviousOutputs()
	if original[0].Name != "plan" {
		t.Errorf("internal state was mutated via PreviousOutputs(): name = %q, want %q",
			original[0].Name, "plan")
	}
}

func TestContextStore_Snapshot(t *testing.T) {
	store := newTestContextStore()
	store.Append("plan", "plan output")

	snapshot := store.Snapshot()
	if snapshot.RunID != "run_test123" {
		t.Errorf("Snapshot RunID = %q, want %q", snapshot.RunID, "run_test123")
	}
	if len(snapshot.Steps) != 1 {
		t.Fatalf("Snapshot Steps = %d, want 1", len(snapshot.Steps))
	}
	if snapshot.Steps[0].Name != "plan" {
		t.Errorf("Snapshot Steps[0].Name = %q, want %q", snapshot.Steps[0].Name, "plan")
	}
}

func TestContextStore_RestoreFromSnapshot(t *testing.T) {
	original := newTestContextStore()
	original.Append("plan", "plan output")
	original.Append("generate", "generate output")

	snapshot := original.Snapshot()
	restored := run.RestoreFromSnapshot(snapshot)

	outputs := restored.PreviousOutputs()
	if len(outputs) != 2 {
		t.Fatalf("restored should have 2 outputs, got %d", len(outputs))
	}
	if outputs[0].Name != "plan" {
		t.Errorf("restored[0].Name = %q, want %q", outputs[0].Name, "plan")
	}
}

func TestContextStore_Trigger(t *testing.T) {
	store := newTestContextStore()
	td := store.Trigger()
	if td.Intent != "test intent" {
		t.Errorf("Trigger.Intent = %q, want %q", td.Intent, "test intent")
	}
}

func TestContextStore_WriteContextFile(t *testing.T) {
	store := newTestContextStore()
	store.Append("plan", "plan data")

	path, cleanup, err := store.WriteContextFile()
	if err != nil {
		t.Fatalf("WriteContextFile returned error: %v", err)
	}
	defer cleanup()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read context file: %v", err)
	}

	var parsed run.Context
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("context file is not valid JSON: %v", err)
	}

	if parsed.RunID != "run_test123" {
		t.Errorf("parsed RunID = %q, want %q", parsed.RunID, "run_test123")
	}
	if len(parsed.Steps) != 1 {
		t.Fatalf("parsed Steps = %d, want 1", len(parsed.Steps))
	}
	if parsed.Steps[0].Output != "plan data" {
		t.Errorf("parsed Steps[0].Output = %q, want %q", parsed.Steps[0].Output, "plan data")
	}
}

func TestContextStore_WriteContextFile_Cleanup(t *testing.T) {
	store := newTestContextStore()
	path, cleanup, err := store.WriteContextFile()
	if err != nil {
		t.Fatalf("WriteContextFile returned error: %v", err)
	}

	cleanup()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("context file should be removed after cleanup")
	}
}

func TestContextStore_ConcurrentAccess(t *testing.T) {
	store := newTestContextStore()
	var wg sync.WaitGroup

	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Append("step", "output")
			_ = store.PreviousOutputs()
			_ = store.Snapshot()
		}(i)
	}

	wg.Wait()

	snapshot := store.Snapshot()
	if snapshot.RunID != "run_test123" {
		t.Errorf("RunID corrupted after concurrent access: %q", snapshot.RunID)
	}
}
