// Package budget aggregates runtime-reported usage across heterogeneous
// runtimes and enforces declared caps. Jerry keeps no price tables: cost is
// whatever the runtime reported. Every attempt counts — money spent is
// spent, and the ledger is the audit record of it.
package budget

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/kilupskalvis/jerry/internal/runtime"
	"github.com/kilupskalvis/jerry/internal/spec"
)

// Entry is one recorded attempt.
type Entry struct {
	Step  string        `json:"step"`
	Usage runtime.Usage `json:"usage"`
}

// Ledger is the file-backed usage record for one run.
type Ledger struct {
	path    string
	Entries []Entry `json:"entries"`
}

// Load reads the ledger at path, or starts an empty one if absent.
func Load(path string) (*Ledger, error) {
	l := &Ledger{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading ledger: %w", err)
	}
	if err := json.Unmarshal(data, l); err != nil {
		return nil, fmt.Errorf("corrupt ledger %s: %w", path, err)
	}
	return l, nil
}

// Record appends an attempt's usage.
func (l *Ledger) Record(step string, u runtime.Usage) {
	l.Entries = append(l.Entries, Entry{Step: step, Usage: u})
}

// Save persists the ledger.
func (l *Ledger) Save() error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling ledger: %w", err)
	}
	return os.WriteFile(l.path, data, 0o644)
}

// Totals returns run-wide cost and token sums.
func (l *Ledger) Totals() (cost float64, tokens int64) {
	for _, e := range l.Entries {
		cost += e.Usage.CostUSD
		tokens += e.Usage.InputTokens + e.Usage.OutputTokens
	}
	return cost, tokens
}

// StepTotals returns one step's cost and token sums across attempts.
func (l *Ledger) StepTotals(step string) (cost float64, tokens int64) {
	for _, e := range l.Entries {
		if e.Step != step {
			continue
		}
		cost += e.Usage.CostUSD
		tokens += e.Usage.InputTokens + e.Usage.OutputTokens
	}
	return cost, tokens
}

// CheckStep returns an error when the step's accumulated usage breaches
// its declared budget. Zero caps mean uncapped.
func (l *Ledger) CheckStep(step string, b spec.Budget) error {
	cost, tokens := l.StepTotals(step)
	if b.MaxCost > 0 && cost > b.MaxCost {
		return fmt.Errorf("step %q spent $%.4f, exceeding max_cost $%.2f", step, cost, b.MaxCost)
	}
	if b.MaxTokens > 0 && tokens > b.MaxTokens {
		return fmt.Errorf("step %q used %d tokens, exceeding max_tokens %d", step, tokens, b.MaxTokens)
	}
	return nil
}

// CheckRun returns an error when run-wide cost breaches the org ceiling.
// Zero ceiling means uncapped.
func (l *Ledger) CheckRun(maxCost float64) error {
	if maxCost <= 0 {
		return nil
	}
	cost, _ := l.Totals()
	if cost > maxCost {
		return fmt.Errorf("run spent $%.4f, exceeding settings ceiling $%.2f", cost, maxCost)
	}
	return nil
}
