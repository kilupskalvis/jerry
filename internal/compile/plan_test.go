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
	if len(plan.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(plan.Files))
	}

	f := plan.Files[0]
	if f.Path != ".github/workflows/jerry-review.yml" {
		t.Errorf("Path = %q", f.Path)
	}
	if len(f.Jobs) != 1 {
		t.Fatalf("len(Jobs) = %d, want 1", len(f.Jobs))
	}

	job := f.Jobs[0]
	if job.Name != "review" {
		t.Errorf("Job.Name = %q", job.Name)
	}
	if job.Triggers.PullRequest == nil {
		t.Error("Triggers.PullRequest is nil")
	}

	// Preamble (4 steps: checkout, install jerry, install pi, drift gate)
	// + 3 workflow steps = 7 total.
	if len(job.Steps) != 7 {
		t.Fatalf("len(Steps) = %d, want 7", len(job.Steps))
	}

	for i := range 4 {
		if !job.Steps[i].IsPreamble {
			t.Errorf("step %d: IsPreamble = false", i)
		}
	}

	s := job.Steps[4]
	if s.Label != "review" {
		t.Errorf("step 4 Label = %q", s.Label)
	}
	if s.Command != "jerry exec review/review" {
		t.Errorf("step 4 Command = %q", s.Command)
	}
	if s.Retries != 0 {
		t.Errorf("step 4 Retries = %d", s.Retries)
	}
}

func TestPlanStepTimeout(t *testing.T) {
	wf := &spec.Workflow{
		Name: "t", Dir: "/tmp/.jerry/t", Version: 1,
		On:    spec.Triggers{Push: &spec.PushTrigger{}},
		Steps: []spec.Step{{Name: "slow", Prompt: "do it", Timeout: spec.Duration{Duration: 600 * time.Second}}},
	}
	plan, err := PlanProject(&spec.Project{
		Root: "/tmp/.jerry", Workflows: []*spec.Workflow{wf},
		Lock: &spec.Lockfile{Version: 1, Runtimes: map[string]spec.LockedRuntime{"pi": {Package: "@mariozechner/pi-coding-agent", Version: "0.73.1"}}},
	}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	s := plan.Files[0].Jobs[0].Steps[4]
	if s.TimeoutMinutes != 11 {
		t.Errorf("TimeoutMinutes = %d, want 11 (ceil(600/60)+1)", s.TimeoutMinutes)
	}
}

func TestPlanStepRetries(t *testing.T) {
	wf := &spec.Workflow{
		Name: "t", Dir: "/tmp/.jerry/t", Version: 1,
		On:    spec.Triggers{Push: &spec.PushTrigger{}},
		Steps: []spec.Step{{Name: "flaky", Prompt: "do it", Retries: 2}},
	}
	plan, err := PlanProject(&spec.Project{
		Root: "/tmp/.jerry", Workflows: []*spec.Workflow{wf},
		Lock: &spec.Lockfile{Version: 1, Runtimes: map[string]spec.LockedRuntime{"pi": {Package: "@mariozechner/pi-coding-agent", Version: "0.73.1"}}},
	}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	s := plan.Files[0].Jobs[0].Steps[4]
	if s.Retries != 2 {
		t.Errorf("Retries = %d, want 2", s.Retries)
	}
}

func TestPlanEnvRefs(t *testing.T) {
	plan, err := PlanProject(reviewProject(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	s := plan.Files[0].Jobs[0].Steps[4]
	if len(s.EnvRefs) != 2 {
		t.Fatalf("len(EnvRefs) = %d, want 2", len(s.EnvRefs))
	}
	if s.EnvRefs[0].Name != "ANTHROPIC_API_KEY" || s.EnvRefs[0].SecretRef != "${{ secrets.ANTHROPIC_API_KEY }}" {
		t.Errorf("EnvRefs[0] = %+v", s.EnvRefs[0])
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
	plan, err := PlanProject(&spec.Project{
		Root: "/tmp/.jerry", Workflows: []*spec.Workflow{wf},
		Lock: &spec.Lockfile{Version: 1, Runtimes: map[string]spec.LockedRuntime{"pi": {Package: "@mariozechner/pi-coding-agent", Version: "0.73.1"}}},
	}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	steps := plan.Files[0].Jobs[0].Steps
	if len(steps[4].EnvRefs) != 2 {
		t.Errorf("agent EnvRefs = %d, want 2", len(steps[4].EnvRefs))
	}
	if len(steps[5].EnvRefs) != 0 {
		t.Errorf("shell EnvRefs = %d, want 0", len(steps[5].EnvRefs))
	}
	if len(steps[6].EnvRefs) != 2 {
		t.Errorf("ci EnvRefs = %d, want 2", len(steps[6].EnvRefs))
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
	if p1.Files[0].Path != p2.Files[0].Path {
		t.Error("non-deterministic file order")
	}
	if p1.Files[0].Path != ".github/workflows/jerry-alpha.yml" {
		t.Errorf("first file = %q, want alpha (sorted)", p1.Files[0].Path)
	}
}

func TestPlanRuntimeInstallFromLock(t *testing.T) {
	wf := &spec.Workflow{
		Name: "t", Dir: "/tmp/.jerry/t", Version: 1,
		On:    spec.Triggers{Push: &spec.PushTrigger{}},
		Steps: []spec.Step{{Name: "a", Prompt: "do"}},
	}
	lock := &spec.Lockfile{
		Version: 1,
		Runtimes: map[string]spec.LockedRuntime{
			"pi": {Package: "@mariozechner/pi-coding-agent", Version: "0.73.1"},
		},
	}
	plan, err := PlanProject(&spec.Project{
		Root: "/tmp/.jerry", Workflows: []*spec.Workflow{wf}, Lock: lock,
	}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	s := plan.Files[0].Jobs[0].Steps[2]
	want := "npm install -g @mariozechner/pi-coding-agent@0.73.1"
	if s.Command != want {
		t.Errorf("install command = %q, want %q", s.Command, want)
	}
}

func TestPlanNoLockfileSkipsRuntimeInstall(t *testing.T) {
	wf := &spec.Workflow{
		Name: "t", Dir: "/tmp/.jerry/t", Version: 1,
		On:    spec.Triggers{Push: &spec.PushTrigger{}},
		Steps: []spec.Step{{Name: "a", Prompt: "do"}},
	}
	plan, err := PlanProject(&spec.Project{
		Root: "/tmp/.jerry", Workflows: []*spec.Workflow{wf}, Lock: nil,
	}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	preambleCount := 0
	for _, s := range plan.Files[0].Jobs[0].Steps {
		if s.IsPreamble {
			preambleCount++
		}
	}
	if preambleCount != 3 {
		t.Errorf("preamble steps = %d, want 3 (no runtime install without lockfile)", preambleCount)
	}
}
