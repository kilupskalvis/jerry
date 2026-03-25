package contextstore_test

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/kilupskalvis/jerry/internal/contextstore"
)

func newTestStore() *contextstore.Store {
	return contextstore.NewStore("run_test123", contextstore.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: "test intent",
	})
}

func TestNewStore(t *testing.T) {
	store := newTestStore()
	snapshot := store.Get()

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
	if snapshot.Data == nil {
		t.Error("Data should be initialized (not nil)")
	}
	if len(snapshot.Data) != 0 {
		t.Errorf("Data should be empty, got %d entries", len(snapshot.Data))
	}
}

func TestStore_SetAndGet(t *testing.T) {
	store := newTestStore()

	err := store.Set("codebase", map[string]any{"language": "go"})
	if err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	snapshot := store.Get()
	codebase, ok := snapshot.Data["codebase"]
	if !ok {
		t.Fatal("Data should contain 'codebase' key")
	}

	cbMap, ok := codebase.(map[string]any)
	if !ok {
		t.Fatalf("codebase should be a map, got %T", codebase)
	}
	if cbMap["language"] != "go" {
		t.Errorf("codebase.language = %q, want %q", cbMap["language"], "go")
	}
}

func TestStore_ReservedKey_Trigger(t *testing.T) {
	store := newTestStore()
	err := store.Set("trigger", "overwrite")
	if err == nil {
		t.Fatal("expected error when writing to reserved key 'trigger'")
	}
}

func TestStore_ReservedKey_RunID(t *testing.T) {
	store := newTestStore()
	err := store.Set("run_id", "overwrite")
	if err == nil {
		t.Fatal("expected error when writing to reserved key 'run_id'")
	}
}

func TestStore_ReservedKey_ProtocolVersion(t *testing.T) {
	store := newTestStore()
	err := store.Set("protocol_version", "overwrite")
	if err == nil {
		t.Fatal("expected error when writing to reserved key 'protocol_version'")
	}
}

func TestStore_GetKeys(t *testing.T) {
	store := newTestStore()
	_ = store.Set("alpha", "a")
	_ = store.Set("beta", "b")
	_ = store.Set("gamma", "c")

	result := store.GetKeys([]string{"alpha", "gamma"})

	if len(result) != 2 {
		t.Fatalf("GetKeys returned %d keys, want 2", len(result))
	}
	if result["alpha"] != "a" {
		t.Errorf("alpha = %v, want %q", result["alpha"], "a")
	}
	if result["gamma"] != "c" {
		t.Errorf("gamma = %v, want %q", result["gamma"], "c")
	}
	if _, ok := result["beta"]; ok {
		t.Error("beta should not be in result")
	}
}

func TestStore_GetKeys_MissingKey(t *testing.T) {
	store := newTestStore()
	_ = store.Set("alpha", "a")

	result := store.GetKeys([]string{"alpha", "nonexistent"})

	if len(result) != 1 {
		t.Fatalf("GetKeys returned %d keys, want 1", len(result))
	}
	if _, ok := result["nonexistent"]; ok {
		t.Error("nonexistent key should be omitted from result")
	}
}

func TestStore_FullReplacement(t *testing.T) {
	store := newTestStore()
	_ = store.Set("data", map[string]any{"version": 1})
	_ = store.Set("data", map[string]any{"version": 2})

	snapshot := store.Get()
	dataMap, ok := snapshot.Data["data"].(map[string]any)
	if !ok {
		t.Fatal("data should be a map")
	}
	// Should be version 2, not merged with version 1
	if v, _ := dataMap["version"].(float64); v != 2 {
		t.Errorf("version = %v, want 2 (full replacement, not merge)", dataMap["version"])
	}
}

func TestStore_DeepCopy(t *testing.T) {
	store := newTestStore()
	_ = store.Set("items", map[string]any{"count": float64(1)})

	// Get a snapshot and mutate it
	snapshot := store.Get()
	items := snapshot.Data["items"].(map[string]any)
	items["count"] = float64(999)

	// Original should be unchanged
	original := store.Get()
	originalItems := original.Data["items"].(map[string]any)
	if originalItems["count"] != float64(1) {
		t.Errorf("internal state was mutated via Get() return value: count = %v, want 1",
			originalItems["count"])
	}
}

func TestStore_Snapshot(t *testing.T) {
	store := newTestStore()
	_ = store.Set("key", "value")

	snapshot := store.Snapshot()
	if snapshot.RunID != "run_test123" {
		t.Errorf("Snapshot RunID = %q, want %q", snapshot.RunID, "run_test123")
	}
	if snapshot.Data["key"] != "value" {
		t.Errorf("Snapshot Data[key] = %v, want %q", snapshot.Data["key"], "value")
	}
}

func TestStore_WriteContextFile(t *testing.T) {
	store := newTestStore()
	_ = store.Set("test", "data")

	path, cleanup, err := store.WriteContextFile()
	if err != nil {
		t.Fatalf("WriteContextFile returned error: %v", err)
	}
	defer cleanup()

	// File should exist
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read context file: %v", err)
	}

	// Should be valid JSON
	var parsed contextstore.Context
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("context file is not valid JSON: %v", err)
	}

	if parsed.RunID != "run_test123" {
		t.Errorf("parsed RunID = %q, want %q", parsed.RunID, "run_test123")
	}
	if parsed.Data["test"] != "data" {
		t.Errorf("parsed Data[test] = %v, want %q", parsed.Data["test"], "data")
	}
}

func TestStore_WriteContextFile_Cleanup(t *testing.T) {
	store := newTestStore()
	path, cleanup, err := store.WriteContextFile()
	if err != nil {
		t.Fatalf("WriteContextFile returned error: %v", err)
	}

	cleanup()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("context file should be removed after cleanup, but still exists at %q", path)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := newTestStore()
	var wg sync.WaitGroup

	// 10 goroutines writing concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key"
			_ = store.Set(key, n)
			_ = store.Get()
			_ = store.GetKeys([]string{key})
		}(i)
	}

	wg.Wait()

	// Should not panic or corrupt — just verify we can still read
	snapshot := store.Get()
	if snapshot.RunID != "run_test123" {
		t.Errorf("RunID corrupted after concurrent access: %q", snapshot.RunID)
	}
}
