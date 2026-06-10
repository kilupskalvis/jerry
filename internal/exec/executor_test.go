package exec

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/handoff"
	"github.com/kilupskalvis/jerry/internal/runtime"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

// testProject writes a minimal v3 project into a temp git repo and returns
// (repoRoot, jerryDir).
func testProject(t *testing.T, workflowYAML, promptMD string) (string, string) {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"commit", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	wfDir := filepath.Join(repo, ".jerry", "wf")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(workflowYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if promptMD != "" {
		if err := os.WriteFile(filepath.Join(wfDir, "agent.md"), []byte(promptMD), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo, filepath.Join(repo, ".jerry")
}

const agentWorkflow = `
version: 1
on: { push: {} }
steps:
  - name: plan
    prompt: agent.md
    outputs:
      approach: string
`

func newTestExecutor(repo, jerryDir string, fake *runtime.Fake) *Executor {
	return New(Options{
		RepoRoot: repo,
		JerryDir: jerryDir,
		Registry: runtime.NewRegistry(fake),
		Out:      io.Discard,
	})
}

func TestAgentStepHappyPath(t *testing.T) {
	repo, jerryDir := testProject(t, agentWorkflow, "Plan: ${{ trigger.intent }}")
	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{
		Text:    "planned",
		Outputs: map[string]any{"approach": "small steps"},
		Usage:   &runtime.Usage{CostUSD: 0.05, InputTokens: 10, OutputTokens: 5},
	})

	e := newTestExecutor(repo, jerryDir, fake)
	ctxDir := filepath.Join(repo, ".jerry-run")
	code := e.Run(context.Background(), Request{
		Workflow: "wf", Step: "plan", CtxDir: ctxDir,
		Trigger: &trigger.TriggerData{Type: "manual", Source: "cli", Intent: "add pagination"},
	})
	if code != 0 {
		t.Fatalf("Run exit = %d, want 0", code)
	}

	if len(fake.Invocations) != 1 {
		t.Fatalf("invocations = %d", len(fake.Invocations))
	}
	inv := fake.Invocations[0]
	if inv.OutputSchema["approach"] != "string" {
		t.Errorf("schema not passed: %+v", inv.OutputSchema)
	}
	if !strings.Contains(inv.Prompt, "add pagination") {
		t.Errorf("trigger intent not in prompt:\n%s", inv.Prompt)
	}

	dir := handoff.NewCtxDir(ctxDir)
	rec, err := dir.ReadStep("plan")
	if err != nil {
		t.Fatalf("ReadStep: %v", err)
	}
	if rec.Output != "planned" || rec.Outputs["approach"] != "small steps" {
		t.Errorf("step record = %+v", rec)
	}
}
