package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/pipeline"
	"github.com/kilupskalvis/jerry/internal/testutil"
)

func TestLoad_ValidBasic(t *testing.T) {
	repoRoot := testutil.SetupTestJerryDir(t, map[string]string{
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
	jerryDir := filepath.Join(repoRoot, ".jerry")
	loader := pipeline.NewLoader(jerryDir)

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
	repoRoot := testutil.SetupTestJerryDir(t, map[string]string{
		"retries": `
name: retries
steps:
  - name: flaky
    script: exit 1
    retries: 2
    retry_backoff: exponential
`,
	})
	jerryDir := filepath.Join(repoRoot, ".jerry")
	loader := pipeline.NewLoader(jerryDir)

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
	repoRoot := testutil.SetupTestJerryDir(t, map[string]string{
		"test": `
name: test
steps:
  - name: s
    script: echo hi
`,
	})
	jerryDir := filepath.Join(repoRoot, ".jerry")
	loader := pipeline.NewLoader(jerryDir)

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
	repoRoot := testutil.SetupTestJerryDir(t, nil)
	jerryDir := filepath.Join(repoRoot, ".jerry")

	// Write a .yml file manually
	pipelinesDir := filepath.Join(jerryDir, "pipelines")
	content := []byte("name: yml-test\nsteps:\n  - name: s\n    script: echo hi\n")
	if err := os.WriteFile(filepath.Join(pipelinesDir, "alt.yml"), content, 0o644); err != nil {
		t.Fatalf("failed to write yml file: %v", err)
	}

	loader := pipeline.NewLoader(jerryDir)
	p, err := loader.Load("alt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "yml-test" {
		t.Errorf("Name = %q, want %q", p.Name, "yml-test")
	}
}

func TestLoad_PipelineNotFound(t *testing.T) {
	repoRoot := testutil.SetupTestJerryDir(t, nil)
	jerryDir := filepath.Join(repoRoot, ".jerry")
	loader := pipeline.NewLoader(jerryDir)

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

// Helper

func assertValidationError(t *testing.T, yamlContent, expectedSubstring string) {
	t.Helper()

	repoRoot := testutil.SetupTestJerryDir(t, map[string]string{
		"test": yamlContent,
	})
	jerryDir := filepath.Join(repoRoot, ".jerry")
	loader := pipeline.NewLoader(jerryDir)

	_, err := loader.Load("test")
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("error %q should contain %q", err.Error(), expectedSubstring)
	}
}
