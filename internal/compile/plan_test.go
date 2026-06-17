package compile

import (
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/spec"
)

func reviewWorkflow() *spec.Workflow {
	return &spec.Workflow{
		Name:    "review",
		Dir:     "/tmp/.jerry/review",
		Version: 1,
		On: spec.Triggers{
			PullRequest: &spec.PullRequestTrigger{Types: []string{"opened", "synchronize"}},
		},
		Env:      []string{"ANTHROPIC_API_KEY", "GITHUB_TOKEN"},
		Defaults: spec.Defaults{Model: "claude-sonnet-4-6"},
		Steps: []spec.Step{
			{
				Name:   "review",
				Prompt: "reviewer.md",
				Model:  "claude-sonnet-4-6",
				Permissions: spec.PermissionSet{
					Allow: []string{"read", "bash(go test:*)", "bash(go vet:*)"},
				},
				Budget:  spec.Budget{MaxCost: 1.50},
				Outputs: map[string]string{"verdict": "string", "findings": "string"},
			},
			{
				Name: "report",
				CI:   "post_pr_comment",
				Body: "## Jerry Review\n${{ steps.review.outputs.findings }}",
			},
			{
				Name:   "gate",
				CI:     "add_check_status",
				Status: "${{ steps.review.outputs.verdict }}",
			},
		},
	}
}

func reviewProject() *spec.Project {
	return &spec.Project{
		Root:      "/tmp/.jerry",
		Workflows: []*spec.Workflow{reviewWorkflow()},
		Lock: &spec.Lockfile{
			Version:  1,
			Runtimes: map[string]spec.LockedRuntime{"pi": {Package: "@mariozechner/pi-coding-agent", Version: "0.73.1"}},
		},
	}
}

func TestPlanReviewWorkflow(t *testing.T) {
	plan, err := PlanProject(reviewProject(), "0.1.0")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.JerryVersion != "0.1.0" {
		t.Errorf("JerryVersion = %q", plan.JerryVersion)
	}
	if len(plan.Workflows) != 1 {
		t.Fatalf("len(Workflows) = %d, want 1", len(plan.Workflows))
	}

	pw := plan.Workflows[0]
	if pw.Name != "review" {
		t.Errorf("Name = %q", pw.Name)
	}
	if pw.Triggers.PullRequest == nil {
		t.Error("Triggers.PullRequest is nil")
	}
	if len(pw.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(pw.Steps))
	}

	s := pw.Steps[0]
	if s.Label != "review" {
		t.Errorf("step 0 Label = %q", s.Label)
	}
	if s.Command != "jerry exec review/review" {
		t.Errorf("step 0 Command = %q", s.Command)
	}
}

func TestPlanStepTimeout(t *testing.T) {
	wf := &spec.Workflow{
		Name: "t", Dir: "/tmp/.jerry/t", Version: 1,
		On:    spec.Triggers{Push: &spec.PushTrigger{}},
		Steps: []spec.Step{{Name: "slow", Prompt: "do it", Timeout: spec.Duration{Duration: 600 * time.Second}}},
	}
	plan, _ := PlanProject(&spec.Project{Root: "/tmp/.jerry", Workflows: []*spec.Workflow{wf}}, "dev")
	s := plan.Workflows[0].Steps[0]
	if s.TimeoutMinutes != 11 {
		t.Errorf("TimeoutMinutes = %d, want 11", s.TimeoutMinutes)
	}
}

func TestPlanStepRetries(t *testing.T) {
	wf := &spec.Workflow{
		Name: "t", Dir: "/tmp/.jerry/t", Version: 1,
		On:    spec.Triggers{Push: &spec.PushTrigger{}},
		Steps: []spec.Step{{Name: "flaky", Prompt: "do it", Retries: 2}},
	}
	plan, _ := PlanProject(&spec.Project{Root: "/tmp/.jerry", Workflows: []*spec.Workflow{wf}}, "dev")
	if plan.Workflows[0].Steps[0].Retries != 2 {
		t.Error("Retries wrong")
	}
}

func TestPlanEnvNames(t *testing.T) {
	plan, _ := PlanProject(reviewProject(), "dev")
	s := plan.Workflows[0].Steps[0]
	if len(s.EnvNames) != 2 {
		t.Fatalf("len(EnvNames) = %d, want 2", len(s.EnvNames))
	}
	if s.EnvNames[0] != "ANTHROPIC_API_KEY" {
		t.Errorf("EnvNames[0] = %q", s.EnvNames[0])
	}
}

func TestPlanStepEnvNarrowing(t *testing.T) {
	empty := []string{}
	wf := &spec.Workflow{
		Name: "t", Dir: "/tmp/.jerry/t", Version: 1,
		On:  spec.Triggers{Push: &spec.PushTrigger{}},
		Env: []string{"ANTHROPIC_API_KEY", "GITHUB_TOKEN"},
		Steps: []spec.Step{
			{Name: "agent", Prompt: "do", Env: nil},
			{Name: "shell", Run: "echo hi", Env: &empty},
			{Name: "ci", CI: "post_pr_comment", Body: "hi"},
		},
	}
	plan, _ := PlanProject(&spec.Project{Root: "/tmp/.jerry", Workflows: []*spec.Workflow{wf}}, "dev")
	steps := plan.Workflows[0].Steps
	if len(steps[0].EnvNames) != 2 {
		t.Errorf("agent EnvNames = %d", len(steps[0].EnvNames))
	}
	if len(steps[1].EnvNames) != 0 {
		t.Errorf("shell EnvNames = %d", len(steps[1].EnvNames))
	}
	if len(steps[2].EnvNames) != 2 {
		t.Errorf("ci EnvNames = %d", len(steps[2].EnvNames))
	}
}

func TestPlanSortedDeterministic(t *testing.T) {
	project := &spec.Project{
		Root: "/tmp/.jerry",
		Workflows: []*spec.Workflow{
			{Name: "zebra", Dir: "/tmp/.jerry/zebra", Version: 1,
				On:    spec.Triggers{Push: &spec.PushTrigger{}},
				Steps: []spec.Step{{Name: "a", Run: "echo"}}},
			{Name: "alpha", Dir: "/tmp/.jerry/alpha", Version: 1,
				On:    spec.Triggers{Push: &spec.PushTrigger{}},
				Steps: []spec.Step{{Name: "b", Run: "echo"}}},
		},
	}
	p1, _ := PlanProject(project, "dev")
	p2, _ := PlanProject(project, "dev")
	if p1.Workflows[0].Name != p2.Workflows[0].Name {
		t.Error("non-deterministic order")
	}
	if p1.Workflows[0].Name != "alpha" {
		t.Errorf("first = %q, want alpha (sorted)", p1.Workflows[0].Name)
	}
}

func TestPlanLockfileCarried(t *testing.T) {
	plan, _ := PlanProject(reviewProject(), "dev")
	if plan.Lock == nil {
		t.Fatal("Lock not carried into Plan")
	}
	if plan.Lock.Runtimes["pi"].Version != "0.73.1" {
		t.Error("Lock version wrong")
	}
}
