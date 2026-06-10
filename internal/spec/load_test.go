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
