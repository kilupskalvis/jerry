package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/runtime"
)

func v3Project(t *testing.T) (string, string) {
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
	wfDir := filepath.Join(repo, ".jerry", "demo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wf := `
version: 1
on: { push: {} }
steps:
  - name: plan
    prompt: "Plan: ${{ trigger.intent }}"
  - name: echo
    run: echo "got $JERRY_INTENT"
`
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wf), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, filepath.Join(repo, ".jerry")
}

func TestRunLocalLoop(t *testing.T) {
	repo, jerryDir := v3Project(t)
	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{Text: "the plan"})

	app := &App{
		JerryDir: jerryDir,
		RepoRoot: repo,
		Registry: runtime.NewRegistry(fake),
		Printer:  output.NewPrinter(io.Discard, io.Discard),
	}
	if err := runLocal(app, "demo", "ship the feature", false); err != nil {
		t.Fatalf("runLocal: %v", err)
	}
	if len(fake.Invocations) != 1 {
		t.Errorf("agent invoked %d times", len(fake.Invocations))
	}
}

func TestRunLocalLoopStepFailureStops(t *testing.T) {
	repo, jerryDir := v3Project(t)
	wfPath := filepath.Join(jerryDir, "demo", "workflow.yaml")
	bad := `
version: 1
on: { push: {} }
steps:
  - name: boom
    run: exit 7
  - name: never
    run: echo unreachable
`
	if err := os.WriteFile(wfPath, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	app := &App{JerryDir: jerryDir, RepoRoot: repo,
		Registry: runtime.NewRegistry(runtime.NewFake("pi")),
		Printer:  output.NewPrinter(io.Discard, io.Discard)}
	if err := runLocal(app, "demo", "x", false); err == nil {
		t.Fatal("want error from failing step")
	}
}

func TestRunLocalBudgetBreachStops(t *testing.T) {
	repo, jerryDir := v3Project(t)

	budgetWF := `
version: 1
on: { push: {} }
steps:
  - name: plan
    prompt: "Plan it"
    budget: { max_cost: 0.10 }
  - name: next
    run: echo "should not run"
`
	if err := os.WriteFile(filepath.Join(jerryDir, "demo", "workflow.yaml"), []byte(budgetWF), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{Text: "expensive", Usage: &runtime.Usage{CostUSD: 0.50}})

	app := &App{JerryDir: jerryDir, RepoRoot: repo,
		Registry: runtime.NewRegistry(fake),
		Printer:  output.NewPrinter(io.Discard, io.Discard)}
	err := runLocal(app, "demo", "x", false)
	if err == nil {
		t.Fatal("want error from budget breach")
	}
	if !strings.Contains(err.Error(), "exit 4") {
		t.Errorf("error = %v, want mention of exit 4", err)
	}
}
