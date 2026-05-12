package validation_test

import (
	"testing"

	"github.com/kilupskalvis/jerry/internal/validation"
)

func TestCheckWorkflowFields_Valid(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"steps": []any{
			map[string]any{"agent": "reviewer"},
		},
	}
	errs := validation.CheckWorkflowFields(raw)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestCheckWorkflowFields_UnknownField(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"steps": []any{},
		"tols":  "bash",
	}
	errs := validation.CheckWorkflowFields(raw)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "tols" {
		t.Errorf("field = %q, want 'tols'", errs[0].Field)
	}
}

func TestCheckWorkflowFields_UnknownStepField(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"steps": []any{
			map[string]any{"agent": "reviewer", "retrys": 2},
		},
	}
	errs := validation.CheckWorkflowFields(raw)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Suggestion != "retries" {
		t.Errorf("suggestion = %q, want 'retries'", errs[0].Suggestion)
	}
}

func TestCheckAgentFields_Valid(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"name":  "reviewer",
		"model": "claude-sonnet-4-6",
		"tools": []any{"post_pr_comment"},
	}
	errs := validation.CheckAgentFields(raw)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestCheckAgentFields_UnknownField(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"name":       "reviewer",
		"model":      "claude-sonnet-4-6",
		"temprature": 0.5,
	}
	errs := validation.CheckAgentFields(raw)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Suggestion != "temperature" {
		t.Errorf("suggestion = %q, want 'temperature'", errs[0].Suggestion)
	}
}

func TestCheckAgentFields_BadTemperature(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"name":        "reviewer",
		"model":       "claude-sonnet-4-6",
		"temperature": "high",
	}
	errs := validation.CheckAgentFields(raw)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestCheckAgentFields_BadMaxIterations(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"name":           "reviewer",
		"model":          "claude-sonnet-4-6",
		"max_iterations": "many",
	}
	errs := validation.CheckAgentFields(raw)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestCheckAgentFields_BadProvider(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"name":     "reviewer",
		"model":    "my-model",
		"provider": "deepseek",
	}
	errs := validation.CheckAgentFields(raw)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestCheckAgentFields_BadToolsType(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"name":  "reviewer",
		"model": "claude-sonnet-4-6",
		"tools": "bash",
	}
	errs := validation.CheckAgentFields(raw)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}
