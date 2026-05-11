package integration_test

import (
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/cli"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

func TestGoldenPath_InitThenRun(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()

	if err := cli.Scaffold(repoRoot); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	env := &testEnv{
		t:        t,
		repoRoot: repoRoot,
		jerryDir: repoRoot + "/.jerry",
		wfName:   "review",
		provider: newScriptedProvider(),
		triggerData: &trigger.TriggerData{
			Type:   "pull_request",
			Source: "github",
			Intent: "Add retry logic",
		},
	}
	env.provider.responses = []llm.CompleteResponse{
		textResponse("Code review: no issues found."),
	}

	result := env.run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Errorf("status = %q, want %q", result.runState.Status, run.StatusCompleted)
	}
	if len(result.llmCalls) == 0 {
		t.Fatal("expected at least 1 LLM call")
	}
	if !strings.Contains(result.llmCalls[0].SystemPrompt, "Add retry logic") {
		t.Error("agent system prompt missing trigger intent")
	}
}

func TestOutputRouting_PRComment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping output routing test in short mode")
	}
	t.Parallel()

	result := newTestEnv(t).
		withWorkflow("comment", "steps:\n  - agent: commenter\n").
		withAgent("comment", "commenter.md", `---
name: commenter
model: claude-sonnet-4-6
tools:
  - post_pr_comment
---
Post a review comment on the PR.
`).
		withTrigger(trigger.TriggerData{
			Type:      "pull_request",
			Source:    "github",
			Intent:    "Fix auth bug",
			Number:    42,
			HeadSHA:   "abc123",
			RepoOwner: "testorg",
			RepoName:  "testrepo",
		}).
		withGitHubAPI().
		withLLMResponses(
			toolCallResponse("post_pr_comment", `{"body": "LGTM, no issues found."}`),
			textResponse("Review posted."),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if len(result.httpReqs) != 1 {
		t.Fatalf("got %d HTTP requests, want 1", len(result.httpReqs))
	}

	req := result.httpReqs[0]
	if req.Method != "POST" {
		t.Errorf("method = %q, want POST", req.Method)
	}
	wantPath := "/repos/testorg/testrepo/issues/42/comments"
	if req.Path != wantPath {
		t.Errorf("path = %q, want %q", req.Path, wantPath)
	}
	body, ok := req.Body["body"].(string)
	if !ok || body != "LGTM, no issues found." {
		t.Errorf("body = %v, want %q", req.Body["body"], "LGTM, no issues found.")
	}
}

func TestTriggerFile_GitHubPR(t *testing.T) {
	t.Parallel()

	githubWebhook := `{
		"action": "opened",
		"pull_request": {
			"number": 77,
			"title": "Fix timeout in retry loop",
			"html_url": "https://github.com/myorg/myrepo/pull/77",
			"user": {"login": "bob"},
			"head": {"sha": "deadbeef123"}
		},
		"repository": {
			"name": "myrepo",
			"owner": {"login": "myorg"}
		}
	}`

	result := newTestEnv(t).
		withWorkflow("review", "steps:\n  - agent: reviewer\n").
		withAgent("review", "reviewer.md", `---
name: reviewer
model: claude-sonnet-4-6
---
Review the PR.
`).
		withTriggerFile(githubWebhook).
		withLLMResponses(textResponse("PR looks good.")).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}

	prompt := result.llmCalls[0].SystemPrompt
	for _, want := range []string{
		"Type: pull_request",
		"Source: github",
		"Intent: Fix timeout in retry loop",
		"Author: bob",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestTriggerJira_FeatureWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-step workflow test in short mode")
	}
	t.Parallel()

	jiraDispatch := `{
		"action": "jerry-ticket",
		"client_payload": {
			"type": "ticket",
			"source": "jira",
			"intent": "Add dark mode support",
			"raw_payload": {
				"key": "PROJ-123",
				"summary": "Add dark mode support",
				"description": "Users want a dark theme option in settings"
			}
		}
	}`

	result := newTestEnv(t).
		withWorkflow("feature", "steps:\n  - agent: planner\n  - agent: generator\n  - name: verify\n    run: cat \"$JERRY_CONTEXT_FILE\"\n").
		withAgent("feature", "planner.md", `---
name: planner
model: claude-sonnet-4-6
---
Plan the feature implementation.
`).
		withAgent("feature", "generator.md", `---
name: generator
model: claude-sonnet-4-6
---
Generate code based on the plan.
`).
		withTriggerFile(jiraDispatch).
		withLLMResponses(
			textResponse("Plan: add theme toggle in settings page."),
			textResponse("Generated: ThemeToggle component with dark/light modes."),
		).
		run()

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.runState.Status != run.StatusCompleted {
		t.Fatalf("status = %q, want %q", result.runState.Status, run.StatusCompleted)
	}
	if len(result.llmCalls) < 2 {
		t.Fatalf("got %d LLM calls, want at least 2", len(result.llmCalls))
	}

	for i, call := range result.llmCalls[:2] {
		if !strings.Contains(call.SystemPrompt, "Type: ticket") {
			t.Errorf("agent %d missing trigger type 'ticket'", i)
		}
		if !strings.Contains(call.SystemPrompt, "Source: jira") {
			t.Errorf("agent %d missing trigger source 'jira'", i)
		}
		if !strings.Contains(call.SystemPrompt, "Add dark mode support") {
			t.Errorf("agent %d missing trigger intent", i)
		}
	}

	if !strings.Contains(result.llmCalls[1].SystemPrompt, "theme toggle in settings") {
		t.Error("second agent missing first agent's plan output")
	}

	if len(result.runState.StepResults) != 3 {
		t.Fatalf("got %d step results, want 3", len(result.runState.StepResults))
	}
	scriptOutput := result.runState.StepResults[2].Stdout
	if !strings.Contains(scriptOutput, "PROJ-123") {
		t.Errorf("script output missing Jira key, got: %s", scriptOutput)
	}
}
