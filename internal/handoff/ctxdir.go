package handoff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

// CtxDir is the on-disk context bus (.jerry-run/). The filesystem is the
// handoff medium; CI steps in one job share the workspace.
type CtxDir struct {
	root string
}

// NewCtxDir wraps root. Directories are created lazily on write.
func NewCtxDir(root string) *CtxDir { return &CtxDir{root: root} }

// Root returns the context directory root.
func (c *CtxDir) Root() string { return c.root }

// StepRecord is everything one step leaves for later steps.
type StepRecord struct {
	Name     string
	Output   string
	Outputs  map[string]any
	Diff     string
	DiffStat string
	Usage    json.RawMessage
}

func (c *CtxDir) stepDir(name string) string { return filepath.Join(c.root, "steps", name) }

// StepOutputFile returns the path of a step's plain-text output.
func (c *CtxDir) StepOutputFile(name string) string {
	return filepath.Join(c.stepDir(name), "output.txt")
}

// LedgerFile returns the budget ledger path.
func (c *CtxDir) LedgerFile() string { return filepath.Join(c.root, "ledger.json") }

// TriggerFile returns the normalized trigger path.
func (c *CtxDir) TriggerFile() string { return filepath.Join(c.root, "trigger.json") }

// WriteStep persists a step record to steps/<name>/.
func (c *CtxDir) WriteStep(rec StepRecord) error {
	dir := c.stepDir(rec.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	files := map[string][]byte{
		"output.txt": []byte(rec.Output),
	}
	if rec.Outputs != nil {
		data, err := json.MarshalIndent(rec.Outputs, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling outputs for %s: %w", rec.Name, err)
		}
		files["outputs.json"] = data
	}
	if rec.Diff != "" {
		files["diff.patch"] = []byte(rec.Diff)
	}
	if rec.DiffStat != "" {
		files["diff.stat"] = []byte(rec.DiffStat)
	}
	if rec.Usage != nil {
		files["usage.json"] = rec.Usage
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return fmt.Errorf("writing %s for step %s: %w", name, rec.Name, err)
		}
	}
	return nil
}

// ReadStep loads a step record. Missing optional files are empty fields;
// a missing step directory is an error.
func (c *CtxDir) ReadStep(name string) (StepRecord, error) {
	rec := StepRecord{Name: name}
	out, err := os.ReadFile(c.StepOutputFile(name))
	if err != nil {
		return rec, fmt.Errorf("step %q has not run (no output): %w", name, err)
	}
	rec.Output = string(out)

	if data, err := os.ReadFile(filepath.Join(c.stepDir(name), "outputs.json")); err == nil {
		if err := json.Unmarshal(data, &rec.Outputs); err != nil {
			return rec, fmt.Errorf("corrupt outputs.json for step %q: %w", name, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(c.stepDir(name), "diff.patch")); err == nil {
		rec.Diff = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(c.stepDir(name), "diff.stat")); err == nil {
		rec.DiffStat = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(c.stepDir(name), "usage.json")); err == nil {
		rec.Usage = data
	}
	return rec, nil
}

// WriteTrigger persists the normalized trigger (written once per run).
func (c *CtxDir) WriteTrigger(td trigger.TriggerData) error {
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		return fmt.Errorf("creating ctx dir: %w", err)
	}
	data, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling trigger: %w", err)
	}
	return os.WriteFile(c.TriggerFile(), data, 0o644)
}

// ReadTrigger loads the normalized trigger.
func (c *CtxDir) ReadTrigger() (trigger.TriggerData, error) {
	var td trigger.TriggerData
	data, err := os.ReadFile(c.TriggerFile())
	if err != nil {
		return td, fmt.Errorf("trigger not normalized yet: %w", err)
	}
	if err := json.Unmarshal(data, &td); err != nil {
		return td, fmt.Errorf("corrupt trigger.json: %w", err)
	}
	return td, nil
}
