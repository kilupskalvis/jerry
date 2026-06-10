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

func refsWorkflow(t *testing.T, steps string) *Workflow {
	t.Helper()
	return mustParse(t, "version: 1\non: { push: {} }\nsteps:\n"+steps)
}

func TestValidateRefsForwardAndUnknown(t *testing.T) {
	wf := refsWorkflow(t, `
  - name: report
    ci: post_pr_comment
    body: "${{ steps.review.output }} and ${{ steps.later.output }}"
  - name: later
    run: ls
`)
	issues := ValidateWorkflow(wf)
	wantIssue(t, issues, `unknown step "review"`)
	wantIssue(t, issues, `step "later" runs after`)
}

func TestValidateRefsOutputsKey(t *testing.T) {
	wf := refsWorkflow(t, `
  - name: plan
    prompt: "Make a plan"
    outputs: { approach: string }
  - name: report
    ci: post_pr_comment
    body: "${{ steps.plan.outputs.approach }} ${{ steps.plan.outputs.missing }}"
`)
	issues := ValidateWorkflow(wf)
	wantIssue(t, issues, `step "plan" does not declare output "missing"`)
	if len(errorsOf(issues)) != 1 {
		t.Errorf("want exactly 1 error, got %v", errorsOf(issues))
	}
}

func TestValidateRefsStepNameSuggestion(t *testing.T) {
	wf := refsWorkflow(t, `
  - name: plan
    prompt: "Make a plan"
  - name: report
    ci: post_pr_comment
    body: "${{ steps.plna.output }}"
`)
	wantIssue(t, ValidateWorkflow(wf), `did you mean "plan"`)
}

func TestValidateContextEntries(t *testing.T) {
	wf := refsWorkflow(t, `
  - name: implement
    prompt: "Do it"
  - name: review
    prompt: "Review it"
    context: ["trigger", "steps.implement", "diff:implement", "diff:nope", "garbage:x"]
`)
	issues := ValidateWorkflow(wf)
	wantIssue(t, issues, `unknown step "nope"`)
	wantIssue(t, issues, `invalid context entry "garbage:x"`)
	for _, e := range errorsOf(issues) {
		if strings.Contains(e, `"implement"`) {
			t.Errorf("valid entries flagged: %v", e)
		}
	}
}

func TestValidateRefsInPromptFile(t *testing.T) {
	wf, err := LoadWorkflow("testdata/bad-prompt-ref")
	if err != nil {
		t.Fatalf("LoadWorkflow: %v", err)
	}
	wantIssue(t, ValidateWorkflow(wf), `unknown step "ghost"`)
}

func TestValidateTemplateSyntaxError(t *testing.T) {
	wf := refsWorkflow(t, `
  - name: report
    ci: post_pr_comment
    body: "${{ trigger.intent"
`)
	wantIssue(t, ValidateWorkflow(wf), "unterminated")
}

func TestValidateProjectPolicy(t *testing.T) {
	p, err := LoadProject("testdata/project")
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	issues := ValidateProject(p)
	wantIssue(t, issues, `runtime "claude-code" is not allowed by settings.yaml`)
	wantIssue(t, issues, `runtime "claude-code" is not pinned in jerry.lock`)
	found := false
	for _, i := range issues {
		if i.Level == LevelWarning && strings.Contains(i.Message, `"expensive" has no max_cost`) {
			found = true
		}
	}
	if !found {
		t.Errorf("want unbounded-budget warning, got %v", issues)
	}
}

func TestValidateProjectBudgetCeiling(t *testing.T) {
	p, err := LoadProject("testdata/project")
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	p.Settings.Policy.Budget.MaxCostPerRun = 1.00
	wantIssue(t, ValidateProject(p), "declared step budgets total $2.00, exceeding the $1.00 ceiling")
}

func TestValidateProjectNoSettingsNoLock(t *testing.T) {
	p := &Project{Workflows: []*Workflow{mustParseValid(t)}}
	issues := ValidateProject(p)
	if HasErrors(issues) {
		t.Errorf("bare project should validate clean, got %v", errorsOf(issues))
	}
	wantIssue(t, issues, "not pinned")
}

func mustParseValid(t *testing.T) *Workflow {
	t.Helper()
	return mustParse(t, "version: 1\non: { push: {} }\nsteps:\n  - name: a\n    prompt: \"Do something\"\n")
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
