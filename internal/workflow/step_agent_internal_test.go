package workflow

import (
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

func TestBuildSystemPromptWithMetadata(t *testing.T) {
	td := &trigger.TriggerData{
		Type:   "pull_request",
		Source: "github",
		Intent: "Fix auth timeout",
		Author: "kalvis",
		Metadata: map[string]string{
			"description": "Fixes the 30s timeout bug.",
			"base_branch": "main",
			"labels":      "bug, security",
		},
	}

	result := buildSystemPrompt("Review the code.", td, nil)

	for _, want := range []string{
		"Type: pull_request",
		"Source: github",
		"Intent: Fix auth timeout",
		"Author: kalvis",
		"Description: Fixes the 30s timeout bug.",
		"Base branch: main",
		"Labels: bug, security",
		"Review the code.",
	} {
		if !strings.Contains(result, want) {
			t.Errorf("prompt missing %q\ngot:\n%s", want, result)
		}
	}
}

func TestBuildSystemPromptWithoutMetadata(t *testing.T) {
	td := &trigger.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: "Review changes",
	}

	result := buildSystemPrompt("Do the review.", td, nil)

	if !strings.Contains(result, "Type: manual") {
		t.Error("missing trigger type")
	}
	if !strings.Contains(result, "Do the review.") {
		t.Error("missing instructions")
	}
}

func TestBuildSystemPromptMetadataWithPrevOutputs(t *testing.T) {
	td := &trigger.TriggerData{
		Type:   "pull_request",
		Source: "github",
		Metadata: map[string]string{
			"description": "PR body here.",
		},
	}
	prev := []StepOutput{
		{StepName: "plan", Data: "Plan output"},
	}

	result := buildSystemPrompt("Generate code.", td, prev)

	if !strings.Contains(result, "Description: PR body here.") {
		t.Error("missing metadata description")
	}
	if !strings.Contains(result, "### plan") {
		t.Error("missing previous step output")
	}
	if !strings.Contains(result, "Generate code.") {
		t.Error("missing instructions")
	}
}

func TestMetadataDisplayOrder(t *testing.T) {
	td := &trigger.TriggerData{
		Type:   "pull_request",
		Source: "github",
		Metadata: map[string]string{
			"labels":       "bug",
			"description":  "Fix the thing.",
			"head_branch":  "feat/x",
			"base_branch":  "main",
			"custom_field": "extra",
		},
	}

	result := formatTriggerSection(td)

	descIdx := strings.Index(result, "Description:")
	baseIdx := strings.Index(result, "Base branch:")
	headIdx := strings.Index(result, "Head branch:")
	labelsIdx := strings.Index(result, "Labels:")
	customIdx := strings.Index(result, "Custom field:")

	if descIdx > baseIdx {
		t.Error("description should appear before base_branch")
	}
	if baseIdx > headIdx {
		t.Error("base_branch should appear before head_branch")
	}
	if headIdx > labelsIdx {
		t.Error("head_branch should appear before labels")
	}
	if labelsIdx > customIdx {
		t.Error("labels should appear before unknown keys")
	}

	// Run 10 times — order must be stable
	for range 10 {
		if got := formatTriggerSection(td); got != result {
			t.Fatalf("non-deterministic output:\nfirst:\n%s\ngot:\n%s", result, got)
		}
	}
}

func TestMetadataTruncation(t *testing.T) {
	longDesc := strings.Repeat("x", 3000)
	td := &trigger.TriggerData{
		Type:   "pull_request",
		Source: "github",
		Metadata: map[string]string{
			"description": longDesc,
			"labels":      "bug",
		},
	}

	result := formatTriggerSection(td)

	if strings.Contains(result, longDesc) {
		t.Error("full 3000-char description should be truncated")
	}
	if !strings.Contains(result, "... [truncated]") {
		t.Error("truncated description should have truncation marker")
	}
	if !strings.Contains(result, "Labels: bug") {
		t.Error("short metadata should not be truncated")
	}
}

func TestBuildTriggerPrefixWithMetadata(t *testing.T) {
	td := &trigger.TriggerData{
		Type:   "ticket",
		Source: "jira",
		Intent: "Add dark mode",
		Metadata: map[string]string{
			"description": "Users want dark mode support.",
		},
	}

	result := buildTriggerPrefix(td)

	if !strings.Contains(result, "Description: Users want dark mode support.") {
		t.Errorf("subagent trigger prefix missing metadata\ngot:\n%s", result)
	}
}
