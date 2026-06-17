package gitlab

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
		os.MkdirAll(filepath.Dir(golden), 0o755)
		os.WriteFile(golden, got, 0o644)
		t.Logf("updated %s", golden)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden (run -update): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("mismatch %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
	}
}

func piLock() *spec.Lockfile {
	return &spec.Lockfile{
		Version:  1,
		Runtimes: map[string]spec.LockedRuntime{"pi": {Package: "@mariozechner/pi-coding-agent", Version: "0.73.1"}},
	}
}

func TestEmitMinimal(t *testing.T) {
	plan := &compile.Plan{
		JerryVersion: "0.1.0",
		Workflows: []compile.PlannedWorkflow{
			{
				Name:     "minimal",
				Triggers: spec.Triggers{Push: &spec.PushTrigger{Branches: []string{"main"}}},
				Env:      []string{"ANTHROPIC_API_KEY"},
				Steps: []compile.PlannedStep{
					{Label: "greet", Command: "jerry exec minimal/greet",
						EnvNames: []string{"ANTHROPIC_API_KEY"}},
				},
			},
		},
		Lock: piLock(),
	}
	files, err := Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("len = %d", len(files))
	}
	if files[0].Path != ".gitlab-ci-jerry.yml" {
		t.Errorf("path = %q", files[0].Path)
	}
	assertGolden(t, "minimal.yml", files[0].Content)
}

func TestEmitReviewWorkflow(t *testing.T) {
	plan, _ := compile.PlanProject(&spec.Project{
		Root: "/tmp/.jerry",
		Workflows: []*spec.Workflow{
			{
				Name: "review", Dir: "/tmp/.jerry/review", Version: 1,
				On: spec.Triggers{
					PullRequest: &spec.PullRequestTrigger{Types: []string{"opened", "synchronize"}},
				},
				Env: []string{"ANTHROPIC_API_KEY", "GITHUB_TOKEN"},
				Steps: []spec.Step{
					{Name: "review", Prompt: "reviewer.md",
						Outputs: map[string]string{"verdict": "string", "findings": "string"}},
					{Name: "report", CI: "post_pr_comment",
						Body: "## Jerry Review\n${{ steps.review.outputs.findings }}"},
					{Name: "gate", CI: "add_check_status",
						Status: "${{ steps.review.outputs.verdict }}"},
				},
			},
		},
		Lock: piLock(),
	}, "0.1.0")
	files, err := Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "review.yml", files[0].Content)
}

func TestEmitDeterministic(t *testing.T) {
	plan := &compile.Plan{
		JerryVersion: "0.1.0",
		Workflows: []compile.PlannedWorkflow{
			{Name: "a", Triggers: spec.Triggers{Push: &spec.PushTrigger{}},
				Steps: []compile.PlannedStep{{Label: "s", Command: "jerry exec a/s"}}},
		},
	}
	f1, _ := Emit(plan)
	f2, _ := Emit(plan)
	if string(f1[0].Content) != string(f2[0].Content) {
		t.Error("not deterministic")
	}
}
