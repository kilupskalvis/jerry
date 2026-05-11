package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/agent"
)

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
tools:
  - post_pr_comment
---

# Test Agent

You are a test agent.
`)

	loader := agent.NewLoader("")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agentCfg.Name != "test-agent" {
		t.Errorf("name = %q, want 'test-agent'", agentCfg.Name)
	}
	if agentCfg.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want 'claude-sonnet-4-6'", agentCfg.Model)
	}
	if len(agentCfg.Tools) != 1 {
		t.Errorf("tools = %d, want 1", len(agentCfg.Tools))
	}
	if !strings.Contains(agentCfg.Instructions, "You are a test agent") {
		t.Errorf("instructions missing body, got %q", agentCfg.Instructions)
	}
}

func TestLoad_ValidNoTools(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-tools.md", `---
name: reasoning-agent
model: claude-sonnet-4-6
---

# Reasoning Agent

Pure reasoning, no tools.
`)

	loader := agent.NewLoader("")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agentCfg.Tools) != 0 {
		t.Errorf("tools = %d, want 0", len(agentCfg.Tools))
	}
}

func TestLoad_ValidDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "defaults.md", `---
name: defaults-agent
model: claude-sonnet-4-6
---

# Defaults Agent

Test default values.
`)

	loader := agent.NewLoader("")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agentCfg.MaxIterations != agent.DefaultMaxIterations {
		t.Errorf("max_iterations = %d, want %d", agentCfg.MaxIterations, agent.DefaultMaxIterations)
	}
	if agentCfg.Temperature == nil || *agentCfg.Temperature != agent.DefaultTemperature {
		t.Errorf("temperature = %v, want %f", agentCfg.Temperature, agent.DefaultTemperature)
	}
}

func TestLoad_DefaultModelFromLoader(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-model.md", `---
name: no-model-agent
---

# No Model

Agent without model in frontmatter.
`)

	loader := agent.NewLoader("claude-haiku-4-5")
	agentCfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agentCfg.Model != "claude-haiku-4-5" {
		t.Errorf("model = %q, want 'claude-haiku-4-5'", agentCfg.Model)
	}
}

func TestLoad_InvalidNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-front.md", "# Just markdown\n\nNo frontmatter here.")

	loader := agent.NewLoader("")
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

	loader := agent.NewLoader("")
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for bad YAML")
	}
}

func TestLoad_InvalidNoName(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-name.md", `---
model: claude-sonnet-4-6
---

# No Name

Missing name field.
`)

	loader := agent.NewLoader("")
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name', got: %v", err)
	}
}

func TestLoad_InvalidNoInstructions(t *testing.T) {
	dir := t.TempDir()
	path := writeAgentFile(t, dir, "no-instructions.md", `---
name: empty-body-agent
model: claude-sonnet-4-6
---
`)

	loader := agent.NewLoader("")
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
---

# Agent

No model and no default.
`)

	loader := agent.NewLoader("")
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for missing model with no default")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error should mention 'model', got: %v", err)
	}
}
