package exec

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/runtime"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

const ciWorkflow = `
version: 1
on: { push: {} }
steps:
  - name: plan
    prompt: "Plan inline"
    outputs: { verdict: string }
  - name: report
    ci: post_pr_comment
    body: "Verdict was ${{ steps.plan.outputs.verdict }}"
`

func TestCIStepPreview(t *testing.T) {
	repo, jerryDir := testProject(t, ciWorkflow, "")
	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{Text: "ok", Outputs: map[string]any{"verdict": "success"}})

	var out bytes.Buffer
	e := New(Options{RepoRoot: repo, JerryDir: jerryDir,
		Registry: runtime.NewRegistry(fake), Out: &out})

	ctxDir := filepath.Join(repo, ".jerry-run")
	req := Request{Workflow: "wf", Step: "plan", CtxDir: ctxDir,
		Trigger: &trigger.TriggerData{Type: "manual", Source: "cli", Intent: "x"}}
	if code := e.Run(context.Background(), req); code != 0 {
		t.Fatalf("plan exit %d", code)
	}

	req.Step = "report"
	if code := e.Run(context.Background(), req); code != 0 {
		t.Fatalf("report exit %d: %s", code, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "ci preview: post_pr_comment") {
		t.Errorf("missing preview label:\n%s", s)
	}
	if !strings.Contains(s, "Verdict was success") {
		t.Errorf("body not templated:\n%s", s)
	}
}

func TestCIStepLiveUnimplemented(t *testing.T) {
	repo, jerryDir := testProject(t, ciWorkflow, "")
	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{Text: "ok", Outputs: map[string]any{"verdict": "success"}})
	e := newTestExecutor(repo, jerryDir, fake)
	ctxDir := filepath.Join(repo, ".jerry-run")
	req := Request{Workflow: "wf", Step: "plan", CtxDir: ctxDir,
		Trigger: &trigger.TriggerData{Type: "manual", Source: "cli", Intent: "x"}}
	if code := e.Run(context.Background(), req); code != 0 {
		t.Fatal("setup failed")
	}
	req.Step, req.CILive = "report", true
	if code := e.Run(context.Background(), req); code != ExitConfig {
		t.Errorf("exit = %d, want %d (live mode lands in phase 4)", code, ExitConfig)
	}
}
