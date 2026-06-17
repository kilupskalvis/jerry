package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/citools"
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

func TestCIStepLivePostPRComment(t *testing.T) {
	var gotPath string
	var payload map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &payload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	repo, jerryDir := testProject(t, ciWorkflow, "")
	td := &trigger.TriggerData{Type: "pull_request", Source: "github",
		Intent: "review", Number: 42, HeadSHA: "abc123",
		RepoOwner: "acme", RepoName: "widgets"}

	client, err := citools.NewClient(td, citools.Config{Token: "test-tok", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{Text: "ok", Outputs: map[string]any{"verdict": "success"}})

	e := New(Options{RepoRoot: repo, JerryDir: jerryDir,
		Registry: runtime.NewRegistry(fake), Out: io.Discard,
		CIClient: client})

	ctxDir := filepath.Join(repo, ".jerry-run")
	code := e.Run(context.Background(), Request{
		Workflow: "wf", Step: "plan", CtxDir: ctxDir, Trigger: td})
	if code != 0 {
		t.Fatalf("plan exit %d", code)
	}

	code = e.Run(context.Background(), Request{
		Workflow: "wf", Step: "report", CtxDir: ctxDir, Trigger: td, CILive: true})
	if code != 0 {
		t.Fatalf("report exit %d", code)
	}
	if gotPath != "/repos/acme/widgets/issues/42/comments" {
		t.Errorf("API path = %q", gotPath)
	}
	if !strings.Contains(payload["body"], "Verdict was success") {
		t.Errorf("body = %q", payload["body"])
	}
}

const checkWorkflow = `
version: 1
on: { push: {} }
steps:
  - name: plan
    prompt: "Plan inline"
    outputs: { verdict: string }
  - name: gate
    ci: add_check_status
    status: "${{ steps.plan.outputs.verdict }}"
    body: "Jerry check"
`

func TestCIStepLiveAddCheckStatus(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &payload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	repo, jerryDir := testProject(t, checkWorkflow, "")
	td := &trigger.TriggerData{Type: "pull_request", Source: "github",
		Intent: "review", Number: 1, HeadSHA: "sha1",
		RepoOwner: "o", RepoName: "r"}

	client, err := citools.NewClient(td, citools.Config{Token: "tok", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	fake := runtime.NewFake("pi")
	fake.Script(runtime.Result{Text: "ok", Outputs: map[string]any{"verdict": "success"}})

	e := New(Options{RepoRoot: repo, JerryDir: jerryDir,
		Registry: runtime.NewRegistry(fake), Out: io.Discard,
		CIClient: client})

	ctxDir := filepath.Join(repo, ".jerry-run")
	if code := e.Run(context.Background(), Request{
		Workflow: "wf", Step: "plan", CtxDir: ctxDir, Trigger: td}); code != 0 {
		t.Fatalf("plan exit %d", code)
	}

	code := e.Run(context.Background(), Request{
		Workflow: "wf", Step: "gate", CtxDir: ctxDir, Trigger: td, CILive: true})
	if code != 0 {
		t.Fatalf("gate exit %d", code)
	}
	if payload["conclusion"] != "success" || payload["head_sha"] != "sha1" {
		t.Errorf("payload = %v", payload)
	}
}

const reviewCommentWorkflow = `
version: 1
on: { push: {} }
steps:
  - name: annotate
    ci: post_review_comment
    body: "nit: fix this"
`

func TestCIStepLivePostReviewCommentUnsupported(t *testing.T) {
	repo, jerryDir := testProject(t, reviewCommentWorkflow, "")
	td := &trigger.TriggerData{Type: "pull_request", Source: "github",
		Intent: "review", Number: 1, HeadSHA: "sha",
		RepoOwner: "o", RepoName: "r"}

	client, _ := citools.NewClient(td, citools.Config{Token: "tok", BaseURL: "http://unused"})
	e := New(Options{RepoRoot: repo, JerryDir: jerryDir,
		Registry: runtime.NewRegistry(runtime.NewFake("pi")), Out: io.Discard,
		CIClient: client})

	ctxDir := filepath.Join(repo, ".jerry-run")
	code := e.Run(context.Background(), Request{
		Workflow: "wf", Step: "annotate", CtxDir: ctxDir, Trigger: td, CILive: true})
	if code != ExitConfig {
		t.Errorf("exit = %d, want %d (unsupported action)", code, ExitConfig)
	}
}
