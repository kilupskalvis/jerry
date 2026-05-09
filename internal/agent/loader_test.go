package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/agent"
)

var testKnownTools = []string{"read_file", "write_file", "glob", "search_codebase", "run_command", "list_directory"}

func writeAgentFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_ValidBasic(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "basic.md", `---
name: test-agent
model: claude-sonnet-4-6
context_access:
  - trigger
output_key: result
output_schema:
  summary: string
tools:
  - read_file
  - glob
---

# Test Agent

You are a test agent.
`)

	loader := agent.NewLoader(testKnownTools, "")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agentCfg.Name != "test-agent" {
		t.Errorf("expected name 'test-agent', got %q", agentCfg.Name)
	}
	if agentCfg.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model 'claude-sonnet-4-6', got %q", agentCfg.Model)
	}
	if len(agentCfg.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(agentCfg.Tools))
	}
	if !strings.Contains(agentCfg.Instructions, "You are a test agent") {
		t.Errorf("instructions should contain agent body, got %q", agentCfg.Instructions)
	}
}

func TestLoad_ValidNoTools(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-tools.md", `---
name: reasoning-agent
model: claude-sonnet-4-6
context_access:
  - trigger
output_key: analysis
output_schema:
  conclusion: string
---

# Reasoning Agent

Pure reasoning, no tools.
`)

	loader := agent.NewLoader(testKnownTools, "")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agentCfg.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(agentCfg.Tools))
	}
}

func TestLoad_ValidWithConstraints(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "constrained.md", `---
name: constrained-agent
model: claude-sonnet-4-6
context_access:
  - trigger
output_key: result
output_schema:
  done: string
tools:
  - read_file
  - write_file:
      restrict_to:
        - src/
        - tests/
---

# Constrained Agent

Write only to src/ and tests/.
`)

	loader := agent.NewLoader(testKnownTools, "")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agentCfg.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(agentCfg.Tools))
	}
	if agentCfg.Tools[0].Name != "read_file" {
		t.Errorf("first tool should be read_file, got %q", agentCfg.Tools[0].Name)
	}
	if agentCfg.Tools[1].Name != "write_file" {
		t.Errorf("second tool should be write_file, got %q", agentCfg.Tools[1].Name)
	}
	if agentCfg.Tools[1].Constraints == nil {
		t.Fatal("write_file should have constraints")
	}
}

func TestLoad_ValidDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "defaults.md", `---
name: defaults-agent
model: claude-sonnet-4-6
context_access:
  - trigger
output_key: result
output_schema:
  done: string
---

# Defaults Agent

Test default values.
`)

	loader := agent.NewLoader(testKnownTools, "")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agentCfg.MaxIterations != agent.DefaultMaxIterations {
		t.Errorf("expected max_iterations %d, got %d", agent.DefaultMaxIterations, agentCfg.MaxIterations)
	}
	if agentCfg.Temperature == nil || *agentCfg.Temperature != agent.DefaultTemperature {
		t.Errorf("expected temperature %f, got %v", agent.DefaultTemperature, agentCfg.Temperature)
	}
}

func TestLoad_DefaultModelFromLoader(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-model.md", `---
name: no-model-agent
context_access:
  - trigger
output_key: result
output_schema:
  done: string
---

# No Model

Agent without model in frontmatter.
`)

	loader := agent.NewLoader(testKnownTools, "claude-haiku-4-5")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agentCfg.Model != "claude-haiku-4-5" {
		t.Errorf("expected model from default 'claude-haiku-4-5', got %q", agentCfg.Model)
	}
}

func TestLoad_InvalidNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-front.md", "# Just markdown\n\nNo frontmatter here.")

	loader := agent.NewLoader(testKnownTools, "")
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestLoad_InvalidBadYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "bad-yaml.md", `---
name: [invalid yaml
  this is: broken
---

# Bad YAML
`)

	loader := agent.NewLoader(testKnownTools, "")
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for bad YAML")
	}
}

func TestLoad_InvalidNoName(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-name.md", `---
model: claude-sonnet-4-6
context_access:
  - trigger
output_key: result
output_schema:
  done: string
---

# No Name

Missing name field.
`)

	loader := agent.NewLoader(testKnownTools, "")
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name', got: %v", err)
	}
}

func TestLoad_InvalidUnknownTool(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "unknown-tool.md", `---
name: bad-tools-agent
model: claude-sonnet-4-6
context_access:
  - trigger
output_key: result
output_schema:
  done: string
tools:
  - nonexistent_tool
---

# Unknown Tool

References a tool that doesn't exist.
`)

	loader := agent.NewLoader(testKnownTools, "")
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error should mention 'unknown tool', got: %v", err)
	}
}

func TestLoad_InvalidNoInstructions(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-instructions.md", `---
name: empty-body-agent
model: claude-sonnet-4-6
context_access:
  - trigger
output_key: result
output_schema:
  done: string
---
`)

	loader := agent.NewLoader(testKnownTools, "")
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for empty instructions")
	}
	if !strings.Contains(err.Error(), "no instructions") {
		t.Errorf("error should mention 'no instructions', got: %v", err)
	}
}

func TestLoad_InvalidNoModel(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-model.md", `---
name: no-model
context_access:
  - trigger
output_key: result
output_schema:
  done: string
---

# Agent

No model and no default.
`)

	loader := agent.NewLoader(testKnownTools, "") // empty default
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for missing model with no default")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error should mention 'model', got: %v", err)
	}
}
