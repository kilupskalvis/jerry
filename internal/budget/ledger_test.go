package budget

import (
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/runtime"
	"github.com/kilupskalvis/jerry/internal/spec"
)

func TestLedgerRecordAndTotals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	l, err := Load(path)
	if err != nil {
		t.Fatalf("Load fresh: %v", err)
	}

	l.Record("plan", runtime.Usage{InputTokens: 100, OutputTokens: 50, CostUSD: 0.10})
	l.Record("plan", runtime.Usage{InputTokens: 10, OutputTokens: 5, CostUSD: 0.01})
	if err := l.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	l2, err := Load(path)
	if err != nil {
		t.Fatalf("Load existing: %v", err)
	}
	cost, tokens := l2.Totals()
	if cost != 0.11 || tokens != 165 {
		t.Errorf("Totals = %v, %v; want 0.11, 165", cost, tokens)
	}
	if got, _ := l2.StepTotals("plan"); got != 0.11 {
		t.Errorf("StepTotals(plan) = %v", got)
	}
}

func TestLedgerChecks(t *testing.T) {
	l, _ := Load(filepath.Join(t.TempDir(), "ledger.json"))
	l.Record("implement", runtime.Usage{CostUSD: 2.50, OutputTokens: 600000})

	if err := l.CheckStep("implement", spec.Budget{MaxCost: 2.00}); err == nil {
		t.Error("want step cost breach")
	}
	if err := l.CheckStep("implement", spec.Budget{MaxTokens: 500000}); err == nil {
		t.Error("want step token breach")
	}
	if err := l.CheckStep("implement", spec.Budget{MaxCost: 3.00, MaxTokens: 700000}); err != nil {
		t.Errorf("within caps, got %v", err)
	}
	if err := l.CheckRun(2.00); err == nil {
		t.Error("want run ceiling breach")
	}
	if err := l.CheckRun(0); err != nil {
		t.Errorf("zero ceiling = no cap, got %v", err)
	}
}
