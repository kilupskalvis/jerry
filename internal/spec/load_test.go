package spec

import (
	"strings"
	"testing"
)

const minimalYAML = `
version: 1
on:
  pull_request:
    types: [opened, synchronize]
steps:
  - name: review
    prompt: reviewer.md
`

func TestParseWorkflowMinimal(t *testing.T) {
	wf, err := parseWorkflow([]byte(minimalYAML))
	if err != nil {
		t.Fatalf("parseWorkflow: %v", err)
	}
	if wf.Version != 1 {
		t.Errorf("Version = %d, want 1", wf.Version)
	}
	if wf.On.PullRequest == nil {
		t.Fatal("On.PullRequest = nil, want set")
	}
	if got := wf.On.PullRequest.Types; len(got) != 2 || got[0] != "opened" {
		t.Errorf("PullRequest.Types = %v", got)
	}
	if len(wf.Steps) != 1 || wf.Steps[0].Name != "review" || wf.Steps[0].Prompt != "reviewer.md" {
		t.Errorf("Steps = %+v", wf.Steps)
	}
}

func TestParseWorkflowUnknownFieldRejected(t *testing.T) {
	_, err := parseWorkflow([]byte("version: 1\nsteps: []\nbogus_field: 1\n"))
	if err == nil {
		t.Fatal("want error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "bogus_field") {
		t.Errorf("error should name the unknown field, got: %v", err)
	}
}

func TestLoadWorkflowFromDir(t *testing.T) {
	wf, err := LoadWorkflow("testdata/valid-review")
	if err != nil {
		t.Fatalf("LoadWorkflow: %v", err)
	}
	if wf.Name != "valid-review" {
		t.Errorf("Name = %q, want valid-review", wf.Name)
	}
	if wf.Dir == "" {
		t.Error("Dir not set")
	}
	if len(wf.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(wf.Steps))
	}
}

func TestLoadWorkflowMissingFile(t *testing.T) {
	_, err := LoadWorkflow("testdata/does-not-exist")
	if err == nil {
		t.Fatal("want error for missing dir")
	}
}

func TestPromptText(t *testing.T) {
	wf, err := LoadWorkflow("testdata/valid-review")
	if err != nil {
		t.Fatalf("LoadWorkflow: %v", err)
	}

	text, err := wf.PromptText(&wf.Steps[0])
	if err != nil {
		t.Fatalf("PromptText: %v", err)
	}
	if !strings.Contains(text, "Code Reviewer") {
		t.Errorf("file prompt not loaded, got %q", text)
	}

	inline := &Step{Name: "x", Prompt: "Summarize: ${{ steps.review.output }}"}
	text, err = wf.PromptText(inline)
	if err != nil {
		t.Fatalf("PromptText inline: %v", err)
	}
	if text != inline.Prompt {
		t.Errorf("inline prompt mangled: %q", text)
	}

	missing := &Step{Name: "y", Prompt: "nope.md"}
	if _, err := wf.PromptText(missing); err == nil {
		t.Error("want error for missing prompt file")
	}
}

func TestLoadProject(t *testing.T) {
	p, err := LoadProject("testdata/project")
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(p.Workflows) != 1 || p.Workflows[0].Name != "review" {
		t.Errorf("Workflows = %+v", p.Workflows)
	}
	if p.Settings == nil || p.Lock == nil {
		t.Error("settings/lock not loaded")
	}
}

func TestParseWorkflowStepEnvAbsentVsEmpty(t *testing.T) {
	wf, err := parseWorkflow([]byte("version: 1\nsteps:\n  - name: a\n    run: ls\n  - name: b\n    run: ls\n    env: []\n"))
	if err != nil {
		t.Fatalf("parseWorkflow: %v", err)
	}
	if wf.Steps[0].Env != nil {
		t.Error("absent env should be nil (inherit)")
	}
	if wf.Steps[1].Env == nil || len(*wf.Steps[1].Env) != 0 {
		t.Error("explicit empty env should be non-nil empty (narrow to none)")
	}
}
