package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/cli"
	"github.com/kilupskalvis/jerry/internal/run"
)

// newLogsTestApp creates an App with a FileStateStore backed by a temp directory.
func newLogsTestApp(t *testing.T) (*cli.App, *run.FileStateStore) {
	t.Helper()
	runsDir := t.TempDir()
	store := run.NewFileStateStore(runsDir)
	app := &cli.App{
		StateStore: store,
	}
	return app, store
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w

	fn()

	os.Stdout = orig
	_ = w.Close()

	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("io.Copy: %v", copyErr)
	}
	_ = r.Close()

	return buf.String()
}

// seedRun creates a run in the store and returns its RunID.
func seedRun(t *testing.T, store *run.FileStateStore, workflowName string, status run.RunStatus) string {
	t.Helper()
	runID := "run-" + workflowName + "-" + string(status)
	state := run.RunState{
		RunID:        runID,
		WorkflowName: workflowName,
		Status:       status,
		StartedAt:    time.Now(),
		TotalSteps:   2,
	}
	if err := store.InitRun(state); err != nil {
		t.Fatalf("InitRun: %v", err)
	}
	if err := store.SaveFinal(state); err != nil {
		t.Fatalf("SaveFinal: %v", err)
	}
	return runID
}

// seedLogEntry writes a single log entry to the run's log.jsonl file.
func seedLogEntry(t *testing.T, store *run.FileStateStore, runID string, logType run.LogType, step string) {
	t.Helper()
	runDir := store.RunDir(runID)
	writer, err := run.NewLogWriter(runDir)
	if err != nil {
		t.Fatalf("NewLogWriter: %v", err)
	}
	defer func() { _ = writer.Close() }()
	writer.Log(logType, step, map[string]string{"detail": "test"})
}

// --- Tests ---

func TestLogsCmd_NoStateStore(t *testing.T) {
	app := &cli.App{StateStore: nil}
	rootCmd := cli.NewRootCmd(app)
	rootCmd.SetArgs([]string{"logs"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when StateStore is nil, got nil")
	}
}

func TestLogsCmd_Overview_HumanReadable_NoRuns(t *testing.T) {
	app, _ := newLogsTestApp(t)

	stdout := captureStdout(t, func() {
		rootCmd := cli.NewRootCmd(app)
		rootCmd.SetArgs([]string{"logs"})
		_ = rootCmd.Execute()
	})

	if stdout != "" {
		t.Errorf("expected empty stdout for human-readable no-runs, got: %q", stdout)
	}
}

func TestLogsCmd_Overview_JSON_NoRuns(t *testing.T) {
	app, _ := newLogsTestApp(t)

	var execErr error
	stdout := captureStdout(t, func() {
		rootCmd := cli.NewRootCmd(app)
		rootCmd.SetArgs([]string{"logs", "--json"})
		execErr = rootCmd.Execute()
	})

	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty stdout for --json with no runs, got: %q", stdout)
	}
}

func TestLogsCmd_Overview_JSON_WithRuns(t *testing.T) {
	app, store := newLogsTestApp(t)
	seedRun(t, store, "workflow-a", run.StatusCompleted)
	seedRun(t, store, "workflow-b", run.StatusFailed)

	var execErr error
	stdout := captureStdout(t, func() {
		rootCmd := cli.NewRootCmd(app)
		rootCmd.SetArgs([]string{"logs", "--json"})
		execErr = rootCmd.Execute()
	})

	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d: %q", len(lines), stdout)
	}

	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v — %q", i, err, line)
			continue
		}
		if _, ok := obj["run_id"]; !ok {
			t.Errorf("line %d missing run_id field: %q", i, line)
		}
	}
}

func TestLogsCmd_Overview_JSON_RunSummaryFields(t *testing.T) {
	app, store := newLogsTestApp(t)
	runID := seedRun(t, store, "my-workflow", run.StatusCompleted)

	var execErr error
	stdout := captureStdout(t, func() {
		rootCmd := cli.NewRootCmd(app)
		rootCmd.SetArgs([]string{"logs", "--json"})
		execErr = rootCmd.Execute()
	})

	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 NDJSON line, got %d: %q", len(lines), stdout)
	}

	var summary run.RunSummary
	if err := json.Unmarshal([]byte(lines[0]), &summary); err != nil {
		t.Fatalf("failed to unmarshal RunSummary: %v — %q", err, lines[0])
	}

	if summary.RunID != runID {
		t.Errorf("run_id: got %q, want %q", summary.RunID, runID)
	}
	if summary.WorkflowName != "my-workflow" {
		t.Errorf("workflow_name: got %q, want %q", summary.WorkflowName, "my-workflow")
	}
	if summary.Status != run.StatusCompleted {
		t.Errorf("status: got %q, want %q", summary.Status, run.StatusCompleted)
	}
}

func TestLogsCmd_RunDetail_JSON(t *testing.T) {
	app, store := newLogsTestApp(t)
	runID := seedRun(t, store, "detail-workflow", run.StatusCompleted)
	seedLogEntry(t, store, runID, run.LogStepStart, "step-one")
	seedLogEntry(t, store, runID, run.LogStepEnd, "step-one")

	var execErr error
	stdout := captureStdout(t, func() {
		rootCmd := cli.NewRootCmd(app)
		rootCmd.SetArgs([]string{"logs", runID, "--json"})
		execErr = rootCmd.Execute()
	})

	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines (one per log entry), got %d: %q", len(lines), stdout)
	}

	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v — %q", i, err, line)
			continue
		}
		if _, ok := obj["type"]; !ok {
			t.Errorf("line %d missing type field: %q", i, line)
		}
	}
}

func TestLogsCmd_Last_JSON_NoRuns(t *testing.T) {
	app, _ := newLogsTestApp(t)

	var execErr error
	stdout := captureStdout(t, func() {
		rootCmd := cli.NewRootCmd(app)
		rootCmd.SetArgs([]string{"logs", "--last", "--json"})
		execErr = rootCmd.Execute()
	})

	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty stdout for --last --json with no runs, got: %q", stdout)
	}
}

func TestLogsCmd_Last_JSON_WithRun(t *testing.T) {
	app, store := newLogsTestApp(t)
	runID := seedRun(t, store, "last-workflow", run.StatusCompleted)
	seedLogEntry(t, store, runID, run.LogStepStart, "step-one")

	var execErr error
	stdout := captureStdout(t, func() {
		rootCmd := cli.NewRootCmd(app)
		rootCmd.SetArgs([]string{"logs", "--last", "--json"})
		execErr = rootCmd.Execute()
	})

	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 NDJSON line, got %d: %q", len(lines), stdout)
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &obj); err != nil {
		t.Fatalf("line is not valid JSON: %v — %q", err, lines[0])
	}
	if _, ok := obj["type"]; !ok {
		t.Errorf("missing type field in log entry: %q", lines[0])
	}
}

func TestLogsCmd_Last_HumanReadable_NoJSON(t *testing.T) {
	app, store := newLogsTestApp(t)
	seedRun(t, store, "human-workflow", run.StatusCompleted)

	var execErr error
	stdout := captureStdout(t, func() {
		rootCmd := cli.NewRootCmd(app)
		rootCmd.SetArgs([]string{"logs", "--last"})
		execErr = rootCmd.Execute()
	})

	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	// Human-readable output goes to stderr; stdout should be empty.
	if stdout != "" {
		t.Errorf("expected empty stdout for human-readable --last, got: %q", stdout)
	}
}
