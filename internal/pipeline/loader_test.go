package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/motif/internal/pipeline"
	"github.com/kilupskalvis/motif/internal/testutil"
)

func TestLoad_ValidBasic(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"basic": `
name: basic
description: "Basic pipeline"
steps:
  - name: step-one
    script: echo hello
  - name: step-two
    script: echo '{"key":"value"}'
    output_key: result
`,
	})
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	p, err := loader.Load("basic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "basic" {
		t.Errorf("Name = %q, want %q", p.Name, "basic")
	}
	if len(p.Steps) != 2 {
		t.Fatalf("Steps count = %d, want 2", len(p.Steps))
	}
	if p.Steps[1].OutputKey != "result" {
		t.Errorf("Steps[1].OutputKey = %q, want %q", p.Steps[1].OutputKey, "result")
	}
}

func TestLoad_ValidWithRetries(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"retries": `
name: retries
steps:
  - name: flaky
    script: exit 1
    retries: 2
    retry_backoff: exponential
`,
	})
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	p, err := loader.Load("retries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Steps[0].Retries != 2 {
		t.Errorf("Retries = %d, want 2", p.Steps[0].Retries)
	}
	if p.Steps[0].RetryBackoff != "exponential" {
		t.Errorf("RetryBackoff = %q, want %q", p.Steps[0].RetryBackoff, "exponential")
	}
}

func TestLoad_PipelineDiscovery_YAML(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"test": `
name: test
steps:
  - name: s
    script: echo hi
`,
	})
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	// Should find test.yaml when asked for "test"
	p, err := loader.Load("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "test" {
		t.Errorf("Name = %q, want %q", p.Name, "test")
	}
}

func TestLoad_PipelineDiscovery_YML(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, nil)
	motifDir := filepath.Join(repoRoot, ".motif")

	// Write a .yml file manually
	pipelinesDir := filepath.Join(motifDir, "pipelines")
	content := []byte("name: yml-test\nsteps:\n  - name: s\n    script: echo hi\n")
	if err := os.WriteFile(filepath.Join(pipelinesDir, "alt.yml"), content, 0o644); err != nil {
		t.Fatalf("failed to write yml file: %v", err)
	}

	loader := pipeline.NewLoader(motifDir)
	p, err := loader.Load("alt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "yml-test" {
		t.Errorf("Name = %q, want %q", p.Name, "yml-test")
	}
}

func TestLoad_PipelineNotFound(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, nil)
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	_, err := loader.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent pipeline")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got %q", err.Error())
	}
}

// Validation error tests — each exercises one validation rule

func TestLoad_InvalidNoName(t *testing.T) {
	assertValidationError(t, `
steps:
  - name: s
    script: echo hi
`, "must have a 'name' field")
}

func TestLoad_InvalidNoSteps(t *testing.T) {
	assertValidationError(t, `
name: no-steps
`, "must have a 'steps' field")
}

func TestLoad_InvalidEmptySteps(t *testing.T) {
	assertValidationError(t, `
name: empty
steps: []
`, "must have at least one step")
}

func TestLoad_InvalidNoStepName(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - script: echo hi
`, "missing a 'name' field")
}

func TestLoad_InvalidStepNoExecutor(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - name: empty
`, "must define exactly one of")
}

func TestLoad_InvalidDuplicateSteps(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - name: same
    script: echo one
  - name: same
    script: echo two
`, "duplicate step name")
}

func TestLoad_InvalidEmptyScript(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - name: blank
    script: "   "
`, "script must not be empty")
}

func TestLoad_InvalidReservedOutputKey(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - name: s
    script: echo hi
    output_key: trigger
`, "reserved")
}

func TestLoad_InvalidDuplicateOutputKey(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - name: one
    script: echo a
    output_key: same
  - name: two
    script: echo b
    output_key: same
`, "conflicting output_key")
}

func TestLoad_InvalidRetryBackoff(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - name: s
    script: echo hi
    retries: 1
    retry_backoff: random
`, "must be 'fixed' or 'exponential'")
}

func TestLoad_InvalidRetriesNegative(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - name: s
    script: echo hi
    retries: -1
`, "retries must be >= 0")
}

func TestLoad_InvalidFallbackEmptyScript(t *testing.T) {
	assertValidationError(t, `
name: test
steps:
  - name: s
    script: echo hi
    fallback:
      script: "  "
`, "fallback script must not be empty")
}

func TestLoad_WarningsForPhase3Features(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"future": `
name: future
steps:
  - name: cond
    script: echo hi
    if: "true"
  - name: gated
    gate: true
`,
	})
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	results, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for Phase 3+ features")
	}

	hasIfWarning := false
	hasGateWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "conditional") || strings.Contains(w, "if") {
			hasIfWarning = true
		}
		if strings.Contains(w, "gate") {
			hasGateWarning = true
		}
	}
	if !hasIfWarning {
		t.Error("missing warning for 'if' feature")
	}
	if !hasGateWarning {
		t.Error("missing warning for 'gate' feature")
	}
}

func TestLoadAll(t *testing.T) {
	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"one": `
name: one
steps:
  - name: s
    script: echo hi
`,
		"two": `
name: two
steps:
  - name: s
    script: echo hi
`,
	})
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	results, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// Helper

func assertValidationError(t *testing.T, yamlContent, expectedSubstring string) {
	t.Helper()

	repoRoot := testutil.SetupTestMotifDir(t, map[string]string{
		"test": yamlContent,
	})
	motifDir := filepath.Join(repoRoot, ".motif")
	loader := pipeline.NewLoader(motifDir)

	_, err := loader.Load("test")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("error %q should contain %q", err.Error(), expectedSubstring)
	}
}
