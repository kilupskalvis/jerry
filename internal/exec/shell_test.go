package exec

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/handoff"
	"github.com/kilupskalvis/jerry/internal/runtime"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

const shellWorkflow = `
version: 1
on: { push: {} }
steps:
  - name: first
    prompt: "Make output"
  - name: echo
    run: |
      echo "intent=$JERRY_INTENT step=$JERRY_STEP_NAME"
      cat "$JERRY_STEP_FIRST_OUTPUT_FILE"
  - name: fail
    run: exit 3
`

func TestShellStepEnvAndCapture(t *testing.T) {
	repo, jerryDir := testProject(t, shellWorkflow, "")
	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{Text: "first-output"})
	e := newTestExecutor(repo, jerryDir, fake)
	ctxDir := filepath.Join(repo, ".jerry-run")
	req := Request{Workflow: "wf", Step: "first", CtxDir: ctxDir,
		Trigger: &trigger.TriggerData{Type: "manual", Source: "cli", Intent: "ship it"}}
	if code := e.Run(context.Background(), req); code != 0 {
		t.Fatalf("first step exit %d", code)
	}

	req.Step = "echo"
	if code := e.Run(context.Background(), req); code != 0 {
		t.Fatalf("echo step exit %d", code)
	}
	rec, err := handoff.NewCtxDir(ctxDir).ReadStep("echo")
	if err != nil {
		t.Fatalf("ReadStep: %v", err)
	}
	if !strings.Contains(rec.Output, "intent=ship it step=echo") {
		t.Errorf("env contract broken: %q", rec.Output)
	}
	if !strings.Contains(rec.Output, "first-output") {
		t.Errorf("prior step output file not exposed: %q", rec.Output)
	}
}

func TestShellStepNonZeroExit1(t *testing.T) {
	repo, jerryDir := testProject(t, shellWorkflow, "")
	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{Text: "first-output"})
	e := newTestExecutor(repo, jerryDir, fake)
	ctxDir := filepath.Join(repo, ".jerry-run")
	req := Request{Workflow: "wf", Step: "first", CtxDir: ctxDir,
		Trigger: &trigger.TriggerData{Type: "manual", Source: "cli", Intent: "x"}}
	if code := e.Run(context.Background(), req); code != 0 {
		t.Fatal("setup failed")
	}
	req.Step = "fail"
	if code := e.Run(context.Background(), req); code != ExitStep {
		t.Errorf("exit = %d, want %d", code, ExitStep)
	}
}

func TestShellStepNoSecretLeak(t *testing.T) {
	t.Setenv("SUPER_SECRET", "hunter2")
	repo, jerryDir := testProject(t, `
version: 1
on: { push: {} }
steps:
  - name: leak
    run: echo "value=${SUPER_SECRET:-empty}"
`, "")
	e := newTestExecutor(repo, jerryDir, runtime.NewFake("pi"))
	ctxDir := filepath.Join(repo, ".jerry-run")
	if code := e.Run(context.Background(), Request{Workflow: "wf", Step: "leak", CtxDir: ctxDir,
		Trigger: &trigger.TriggerData{Type: "manual", Source: "cli", Intent: "x"}}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	rec, _ := handoff.NewCtxDir(ctxDir).ReadStep("leak")
	if !strings.Contains(rec.Output, "value=empty") {
		t.Errorf("secret leaked into shell step: %q", rec.Output)
	}
}
