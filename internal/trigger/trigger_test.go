package trigger

import (
	"strings"
	"testing"
)

func TestNormalizeGitHub_IssueOpened(t *testing.T) {
	payload := map[string]any{
		"action": "opened",
		"issue": map[string]any{
			"number":   float64(42),
			"title":    "Add notification preferences",
			"body":     "Users should be able to configure notifications",
			"html_url": "https://github.com/org/repo/issues/42",
			"user":     map[string]any{"login": "testuser"},
			"labels": []any{
				map[string]any{"name": "feature"},
				map[string]any{"name": "jerry"},
			},
		},
	}

	trigger, err := NormalizeGitHubEvent("issues.opened", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "ticket" {
		t.Errorf("type = %q, want 'ticket'", trigger.Type)
	}
	if trigger.Source != "github" {
		t.Errorf("source = %q, want 'github'", trigger.Source)
	}
	if trigger.Intent != "Add notification preferences" {
		t.Errorf("intent = %q, want 'Add notification preferences'", trigger.Intent)
	}
	if trigger.Number != 42 {
		t.Errorf("number = %d, want 42", trigger.Number)
	}
}

func TestNormalizeGitHub_Push(t *testing.T) {
	payload := map[string]any{
		"ref": "refs/heads/main",
		"head_commit": map[string]any{
			"message": "update api spec",
			"id":      "abc123def456",
		},
		"sender": map[string]any{"login": "pushuser"},
		"repository": map[string]any{
			"name":  "myrepo",
			"owner": map[string]any{"login": "myorg"},
		},
	}

	trigger, err := NormalizeGitHubEvent("push", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "push" {
		t.Errorf("type = %q, want 'push'", trigger.Type)
	}
	if trigger.Intent != "update api spec" {
		t.Errorf("intent = %q, want 'update api spec'", trigger.Intent)
	}
	if trigger.HeadSHA != "abc123def456" {
		t.Errorf("head_sha = %q, want 'abc123def456'", trigger.HeadSHA)
	}
	if trigger.Author != "pushuser" {
		t.Errorf("author = %q, want 'pushuser'", trigger.Author)
	}
	if trigger.RepoOwner != "myorg" {
		t.Errorf("repo_owner = %q, want 'myorg'", trigger.RepoOwner)
	}
	if trigger.RepoName != "myrepo" {
		t.Errorf("repo_name = %q, want 'myrepo'", trigger.RepoName)
	}
}

func TestNormalizeGitLab_Issue(t *testing.T) {
	payload := map[string]any{
		"object_kind": "issue",
		"user":        map[string]any{"username": "gluser"},
		"object_attributes": map[string]any{
			"title":       "Fix login bug",
			"description": "The login form crashes on empty email",
			"iid":         float64(15),
			"url":         "https://gitlab.com/org/repo/-/issues/15",
		},
		"project": map[string]any{
			"name":      "myproject",
			"namespace": "mygroup",
		},
	}

	trigger, err := NormalizeGitLabEvent("issue", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "ticket" {
		t.Errorf("type = %q, want 'ticket'", trigger.Type)
	}
	if trigger.Source != "gitlab" {
		t.Errorf("source = %q, want 'gitlab'", trigger.Source)
	}
	if trigger.Intent != "Fix login bug" {
		t.Errorf("intent = %q, want 'Fix login bug'", trigger.Intent)
	}
	if trigger.Author != "gluser" {
		t.Errorf("author = %q, want 'gluser'", trigger.Author)
	}
}

func TestNormalizeGitLab_Push(t *testing.T) {
	payload := map[string]any{
		"object_kind":   "push",
		"checkout_sha":  "deadbeef123",
		"user_username": "pusher",
		"project": map[string]any{
			"name":      "myproject",
			"namespace": "mygroup",
		},
		"commits": []any{
			map[string]any{"message": "first commit"},
			map[string]any{"message": "latest commit"},
		},
	}

	trigger, err := NormalizeGitLabEvent("push", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "push" {
		t.Errorf("type = %q, want 'push'", trigger.Type)
	}
	if trigger.Intent != "latest commit" {
		t.Errorf("intent = %q, want 'latest commit'", trigger.Intent)
	}
	if trigger.HeadSHA != "deadbeef123" {
		t.Errorf("head_sha = %q, want 'deadbeef123'", trigger.HeadSHA)
	}
	if trigger.Author != "pusher" {
		t.Errorf("author = %q, want 'pusher'", trigger.Author)
	}
	if trigger.RepoOwner != "mygroup" {
		t.Errorf("repo_owner = %q, want 'mygroup'", trigger.RepoOwner)
	}
	if trigger.RepoName != "myproject" {
		t.Errorf("repo_name = %q, want 'myproject'", trigger.RepoName)
	}
}

func TestFromReader_PreNormalized(t *testing.T) {
	input := `{"type": "manual", "source": "cli", "intent": "add health endpoint"}`
	trigger, err := FromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "manual" {
		t.Errorf("type = %q, want 'manual'", trigger.Type)
	}
	if trigger.Intent != "add health endpoint" {
		t.Errorf("intent = %q, want 'add health endpoint'", trigger.Intent)
	}
}

func TestFromReader_GitHubAutoDetect(t *testing.T) {
	input := `{"action": "opened", "issue": {"number": 1, "title": "test issue", "html_url": "http://test", "user": {"login": "u"}}}`
	trigger, err := FromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "ticket" {
		t.Errorf("type = %q, want 'ticket'", trigger.Type)
	}
	if trigger.Source != "github" {
		t.Errorf("source = %q, want 'github'", trigger.Source)
	}
}

func TestFromReader_RepositoryDispatchWithClientPayload(t *testing.T) {
	input := `{
		"action": "jerry-ticket",
		"client_payload": {
			"type": "ticket",
			"source": "jira",
			"intent": "Add dark mode support",
			"raw_payload": {"key": "PROJ-123"}
		}
	}`
	trigger, err := FromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "ticket" {
		t.Errorf("type = %q, want 'ticket'", trigger.Type)
	}
	if trigger.Source != "jira" {
		t.Errorf("source = %q, want 'jira'", trigger.Source)
	}
	if trigger.Intent != "Add dark mode support" {
		t.Errorf("intent = %q, want 'Add dark mode support'", trigger.Intent)
	}
}

func TestFromReader_RepositoryDispatchWithoutPreNormalized(t *testing.T) {
	input := `{"action": "some-event", "client_payload": {"foo": "bar"}}`
	trigger, err := FromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "webhook" {
		t.Errorf("type = %q, want 'webhook' for non-normalized client_payload", trigger.Type)
	}
}

func TestFromReader_GitHubPushAutoDetect(t *testing.T) {
	input := `{"ref": "refs/heads/main", "head_commit": {"message": "push msg", "id": "sha123"}, "sender": {"login": "u"}, "repository": {"name": "r", "owner": {"login": "o"}}}`
	trigger, err := FromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger.Type != "push" {
		t.Errorf("type = %q, want 'push'", trigger.Type)
	}
	if trigger.Source != "github" {
		t.Errorf("source = %q, want 'github'", trigger.Source)
	}
	if trigger.HeadSHA != "sha123" {
		t.Errorf("head_sha = %q, want 'sha123'", trigger.HeadSHA)
	}
}

func TestFromReader_InvalidJSON(t *testing.T) {
	_, err := FromReader(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
