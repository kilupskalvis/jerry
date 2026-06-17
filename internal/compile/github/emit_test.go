package github

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/compile"
	"github.com/kilupskalvis/jerry/internal/spec"
)

var update = flag.Bool("update", false, "update golden files")

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	golden := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", golden)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to generate): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n--- got ---\n%s\n--- want ---\n%s",
			golden, got, want)
	}
}

func minimalPlan() *compile.Plan {
	return &compile.Plan{
		JerryVersion: "0.1.0",
		Files: []compile.PlannedFile{
			{
				Path: ".github/workflows/jerry-minimal.yml",
				Jobs: []compile.PlannedJob{
					{
						Name:     "minimal",
						Triggers: spec.Triggers{Push: &spec.PushTrigger{Branches: []string{"main"}}},
						Env:      []string{"ANTHROPIC_API_KEY"},
						Steps: []compile.PlannedStep{
							{Label: "Checkout", Command: "actions/checkout@v4", IsPreamble: true},
							{Label: "Install Jerry", Command: "curl -sSL https://jerry.dev/install.sh | sh -s -- --version v0.1.0", IsPreamble: true},
							{Label: "Drift check", Command: "jerry generate --check", IsPreamble: true},
							{Label: "greet", Command: "jerry exec minimal/greet", EnvRefs: []compile.EnvRef{
								{Name: "ANTHROPIC_API_KEY", SecretRef: "${{ secrets.ANTHROPIC_API_KEY }}"},
							}},
						},
					},
				},
			},
		},
	}
}

func TestEmitMinimal(t *testing.T) {
	files, err := Emit(minimalPlan())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	assertGolden(t, "minimal.yml", files[0].Content)
}

func piLock() *spec.Lockfile {
	return &spec.Lockfile{
		Version:  1,
		Runtimes: map[string]spec.LockedRuntime{"pi": {Package: "@mariozechner/pi-coding-agent", Version: "0.73.1"}},
	}
}

func TestEmitReviewWorkflow(t *testing.T) {
	plan, err := compile.PlanProject(&spec.Project{
		Root: "/tmp/.jerry",
		Workflows: []*spec.Workflow{
			{
				Name: "review", Dir: "/tmp/.jerry/review", Version: 1,
				On: spec.Triggers{
					PullRequest: &spec.PullRequestTrigger{Types: []string{"opened", "synchronize"}},
				},
				Env: []string{"ANTHROPIC_API_KEY", "GITHUB_TOKEN"},
				Steps: []spec.Step{
					{Name: "review", Prompt: "reviewer.md", Model: "claude-sonnet-4-6",
						Permissions: spec.PermissionSet{Allow: []string{"read", "bash(go test:*)", "bash(go vet:*)"}},
						Budget:      spec.Budget{MaxCost: 1.50},
						Outputs:     map[string]string{"verdict": "string", "findings": "string"}},
					{Name: "report", CI: "post_pr_comment",
						Body: "## Jerry Review\n${{ steps.review.outputs.findings }}"},
					{Name: "gate", CI: "add_check_status",
						Status: "${{ steps.review.outputs.verdict }}"},
				},
			},
		},
		Lock: piLock(),
	}, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	files, err := Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("len = %d", len(files))
	}
	assertGolden(t, "review.yml", files[0].Content)
}

func TestEmitFeatureWorkflow(t *testing.T) {
	empty := []string{}
	plan, err := compile.PlanProject(&spec.Project{
		Root: "/tmp/.jerry",
		Workflows: []*spec.Workflow{
			{
				Name: "feature", Dir: "/tmp/.jerry/feature", Version: 1,
				On:       spec.Triggers{Dispatch: &spec.DispatchTrigger{Types: []string{"jerry-ticket"}}},
				Env:      []string{"ANTHROPIC_API_KEY", "GITHUB_TOKEN"},
				Defaults: spec.Defaults{Model: "claude-sonnet-4-6"},
				Steps: []spec.Step{
					{Name: "plan", Prompt: "planner.md", Model: "claude-haiku-4-5",
						Budget:  spec.Budget{MaxCost: 0.50},
						Outputs: map[string]string{"approach": "string", "files": "list"}},
					{Name: "implement", Prompt: "generator.md",
						Context:     []string{"trigger", "steps.plan"},
						Permissions: spec.PermissionSet{Allow: []string{"read", "edit", "bash(go test:*)", "bash(go build:*)", "bash(go vet:*)"}},
						Budget:      spec.Budget{MaxCost: 3.00},
						Timeout:     spec.Duration{Duration: 900_000_000_000}},
					{Name: "test", Run: "go test ./...",
						Timeout: spec.Duration{Duration: 300_000_000_000},
						Env:     &empty},
					{Name: "open-pr", CI: "create_pull_request",
						Title: "jerry: ${{ trigger.intent }}",
						Body:  "Automated implementation for: ${{ trigger.intent }}\n\nPlan: ${{ steps.plan.outputs.approach }}\n${{ steps.implement.diff_stat }}"},
				},
			},
		},
		Lock: piLock(),
	}, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	files, err := Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("len = %d", len(files))
	}
	assertGolden(t, "feature.yml", files[0].Content)
}

func TestEmitRetryLoop(t *testing.T) {
	plan := &compile.Plan{
		JerryVersion: "0.1.0",
		Files: []compile.PlannedFile{
			{
				Path: ".github/workflows/jerry-retry.yml",
				Jobs: []compile.PlannedJob{
					{
						Name:     "retry",
						Triggers: spec.Triggers{Push: &spec.PushTrigger{}},
						Steps: []compile.PlannedStep{
							{Label: "Checkout", Command: "actions/checkout@v4", IsPreamble: true},
							{Label: "Drift check", Command: "jerry generate --check", IsPreamble: true},
							{Label: "flaky", Command: "jerry exec retry/flaky",
								Retries:        2,
								TimeoutMinutes: 11,
								EnvRefs: []compile.EnvRef{
									{Name: "ANTHROPIC_API_KEY", SecretRef: "${{ secrets.ANTHROPIC_API_KEY }}"},
								},
							},
						},
					},
				},
			},
		},
	}

	files, err := Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "retry.yml", files[0].Content)
}

func TestEmitDeterministic(t *testing.T) {
	plan := minimalPlan()
	f1, _ := Emit(plan)
	f2, _ := Emit(plan)
	if string(f1[0].Content) != string(f2[0].Content) {
		t.Error("Emit is not deterministic")
	}
}
