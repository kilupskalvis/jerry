package pipeline_test

import (
	"testing"
	"time"

	"github.com/kilupskalvis/motif/internal/pipeline"
	"gopkg.in/yaml.v3"
)

func TestDuration_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{name: "seconds", input: "30s", expected: 30 * time.Second},
		{name: "minutes", input: "5m", expected: 5 * time.Minute},
		{name: "hours", input: "1h", expected: time.Hour},
		{name: "mixed", input: "1m30s", expected: 90 * time.Second},
		{name: "zero", input: "0s", expected: 0},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "empty", input: `""`, expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap in a struct to test YAML unmarshaling
			yamlInput := "timeout: " + tt.input
			var result struct {
				Timeout pipeline.Duration `yaml:"timeout"`
			}

			err := yaml.Unmarshal([]byte(yamlInput), &result)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Timeout.Duration != tt.expected {
				t.Errorf("Duration = %v, want %v", result.Timeout.Duration, tt.expected)
			}
		})
	}
}

func TestStep_YAMLParsing(t *testing.T) {
	input := `
name: generate
agent: ./agents/generate.md
retries: 2
retry_backoff: exponential
timeout: 300s
output_key: artifacts
`
	var step pipeline.Step
	if err := yaml.Unmarshal([]byte(input), &step); err != nil {
		t.Fatalf("failed to parse step YAML: %v", err)
	}

	if step.Name != "generate" {
		t.Errorf("Name = %q, want %q", step.Name, "generate")
	}
	if step.Agent != "./agents/generate.md" {
		t.Errorf("Agent = %q, want %q", step.Agent, "./agents/generate.md")
	}
	if step.Retries != 2 {
		t.Errorf("Retries = %d, want 2", step.Retries)
	}
	if step.RetryBackoff != "exponential" {
		t.Errorf("RetryBackoff = %q, want %q", step.RetryBackoff, "exponential")
	}
	if step.Timeout.Duration != 300*time.Second {
		t.Errorf("Timeout = %v, want 300s", step.Timeout.Duration)
	}
	if step.OutputKey != "artifacts" {
		t.Errorf("OutputKey = %q, want %q", step.OutputKey, "artifacts")
	}
}

func TestPipeline_YAMLParsing(t *testing.T) {
	input := `
name: feature
description: "Generate features"
steps:
  - name: context
    script: echo hello
  - name: generate
    agent: ./agents/generate.md
`
	var p pipeline.Pipeline
	if err := yaml.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("failed to parse pipeline YAML: %v", err)
	}

	if p.Name != "feature" {
		t.Errorf("Name = %q, want %q", p.Name, "feature")
	}
	if len(p.Steps) != 2 {
		t.Fatalf("Steps length = %d, want 2", len(p.Steps))
	}
	if p.Steps[0].Script != "echo hello" {
		t.Errorf("Steps[0].Script = %q, want %q", p.Steps[0].Script, "echo hello")
	}
	if p.Steps[1].Agent != "./agents/generate.md" {
		t.Errorf("Steps[1].Agent = %q, want %q", p.Steps[1].Agent, "./agents/generate.md")
	}
}

func TestFallbackDef_YAMLParsing(t *testing.T) {
	input := `
name: risky
script: exit 1
fallback:
  script: echo "fallback ran"
`
	var step pipeline.Step
	if err := yaml.Unmarshal([]byte(input), &step); err != nil {
		t.Fatalf("failed to parse step with fallback: %v", err)
	}

	if step.Fallback == nil {
		t.Fatal("Fallback should not be nil")
	}
	if step.Fallback.Script != `echo "fallback ran"` {
		t.Errorf("Fallback.Script = %q, want %q", step.Fallback.Script, `echo "fallback ran"`)
	}
}
