package spec

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Workflow {
	t.Helper()
	wf, err := parseWorkflow([]byte(src))
	if err != nil {
		t.Fatalf("parseWorkflow: %v", err)
	}
	wf.Name, wf.Dir = "test", t.TempDir()
	return wf
}

func errorsOf(issues []Issue) []string {
	var out []string
	for _, i := range issues {
		if i.Level == LevelError {
			out = append(out, i.Message)
		}
	}
	return out
}

func wantIssue(t *testing.T, issues []Issue, substr string) {
	t.Helper()
	for _, i := range issues {
		if strings.Contains(i.Message, substr) {
			return
		}
	}
	t.Errorf("no issue containing %q in %v", substr, issues)
}

func TestValidateVersion(t *testing.T) {
	wf := mustParse(t, "version: 2\non:\n  push: {}\nsteps:\n  - name: a\n    run: ls\n")
	wantIssue(t, ValidateWorkflow(wf), "unsupported spec version 2")
}

func TestValidateNoTrigger(t *testing.T) {
	wf := mustParse(t, "version: 1\nsteps:\n  - name: a\n    run: ls\n")
	wantIssue(t, ValidateWorkflow(wf), "at least one trigger")
}

func TestValidateNoSteps(t *testing.T) {
	wf := mustParse(t, "version: 1\non:\n  push: {}\nsteps: []\n")
	wantIssue(t, ValidateWorkflow(wf), "at least one step")
}

func TestValidateStepNames(t *testing.T) {
	wf := mustParse(t, `
version: 1
on: { push: {} }
steps:
  - name: Do_Stuff
    run: ls
  - name: ok-step
    run: ls
  - name: ok-step
    run: ls
  - run: ls
`)
	issues := ValidateWorkflow(wf)
	wantIssue(t, issues, `invalid step name "Do_Stuff"`)
	wantIssue(t, issues, `duplicate step name "ok-step"`)
	wantIssue(t, issues, "step 4: name is required")
}

func TestValidateStepKindExclusivity(t *testing.T) {
	wf := mustParse(t, `
version: 1
on: { push: {} }
steps:
  - name: both
    prompt: p.md
    run: ls
  - name: neither
    retries: 1
`)
	issues := ValidateWorkflow(wf)
	wantIssue(t, issues, `step "both": exactly one of prompt, run, ci`)
	wantIssue(t, issues, `step "neither": exactly one of prompt, run, ci`)
}

func TestValidateCIAction(t *testing.T) {
	wf := mustParse(t, `
version: 1
on: { push: {} }
steps:
  - name: report
    ci: post_pr_coment
    body: hi
`)
	issues := ValidateWorkflow(wf)
	wantIssue(t, issues, `unknown ci action "post_pr_coment"`)
	wantIssue(t, issues, `did you mean "post_pr_comment"`)
}

func TestValidateFieldPlacement(t *testing.T) {
	wf := mustParse(t, `
version: 1
on: { push: {} }
steps:
  - name: shell
    run: ls
    outputs: { x: string }
  - name: agent
    prompt: p.md
    body: nope
`)
	issues := ValidateWorkflow(wf)
	wantIssue(t, issues, `step "shell": outputs is only valid on agent steps`)
	wantIssue(t, issues, `step "agent": body is only valid on ci steps`)
}

func TestValidateOutputsTypesAndBounds(t *testing.T) {
	wf := mustParse(t, `
version: 1
on: { push: {} }
steps:
  - name: a
    prompt: p.md
    outputs: { verdict: enum }
    retries: -1
    budget: { max_cost: -2 }
`)
	issues := ValidateWorkflow(wf)
	wantIssue(t, issues, `unknown output type "enum"`)
	wantIssue(t, issues, "retries must be >= 0")
	wantIssue(t, issues, "max_cost must be > 0")
}

func TestValidateCleanWorkflowNoErrors(t *testing.T) {
	wf, err := LoadWorkflow("testdata/valid-review")
	if err != nil {
		t.Fatalf("LoadWorkflow: %v", err)
	}
	if errs := errorsOf(ValidateWorkflow(wf)); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}
