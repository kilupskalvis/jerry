package handoff

import (
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

func TestCtxDirStepRoundTrip(t *testing.T) {
	dir := NewCtxDir(t.TempDir())
	rec := StepRecord{
		Name:     "plan",
		Output:   "the plan text",
		Outputs:  map[string]any{"approach": "do it", "files": []any{"a.go"}},
		Diff:     "diff --git a/a.go b/a.go\n",
		DiffStat: "1 file changed",
	}
	if err := dir.WriteStep(rec); err != nil {
		t.Fatalf("WriteStep: %v", err)
	}

	got, err := dir.ReadStep("plan")
	if err != nil {
		t.Fatalf("ReadStep: %v", err)
	}
	if got.Output != rec.Output || got.Outputs["approach"] != "do it" ||
		got.Diff != rec.Diff || got.DiffStat != rec.DiffStat {
		t.Errorf("round trip mismatch: %+v", got)
	}

	if _, err := dir.ReadStep("ghost"); err == nil {
		t.Error("want error for missing step")
	}
}

func TestCtxDirTriggerRoundTrip(t *testing.T) {
	dir := NewCtxDir(t.TempDir())
	if _, err := dir.ReadTrigger(); err == nil {
		t.Fatal("want error before trigger written")
	}
	td := trigger.TriggerData{Type: "manual", Source: "cli", Intent: "do things"}
	if err := dir.WriteTrigger(td); err != nil {
		t.Fatalf("WriteTrigger: %v", err)
	}
	got, err := dir.ReadTrigger()
	if err != nil || got.Intent != "do things" {
		t.Fatalf("ReadTrigger = %+v, %v", got, err)
	}
}

func TestCtxDirPaths(t *testing.T) {
	root := t.TempDir()
	dir := NewCtxDir(root)
	if got := dir.StepOutputFile("plan"); got != filepath.Join(root, "steps", "plan", "output.txt") {
		t.Errorf("StepOutputFile = %q", got)
	}
	if got := dir.LedgerFile(); got != filepath.Join(root, "ledger.json") {
		t.Errorf("LedgerFile = %q", got)
	}
}
