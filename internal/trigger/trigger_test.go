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

func TestNormalizeGitHub_PRMetadata(t *testing.T) {
	payload := map[string]any{
		"action": "opened",
		"pull_request": map[string]any{
			"title":    "Fix auth timeout",
			"number":   float64(42),
			"body":     "This PR fixes the 30s timeout in auth middleware.\n\nLinked to PROJ-123.",
			"html_url": "https://github.com/org/repo/pull/42",
			"user":     map[string]any{"login": "kalvis"},
			"head":     map[string]any{"sha": "abc123", "ref": "feature/auth-fix"},
			"base":     map[string]any{"ref": "main"},
			"labels":   []any{map[string]any{"name": "bug"}, map[string]any{"name": "security"}},
			"draft":    false,
		},
		"repository": map[string]any{
			"name":  "repo",
			"owner": map[string]any{"login": "org"},
		},
	}

	td, err := NormalizeGitHubEvent("pull_request.opened", payload)
	if err != nil {
		t.Fatal(err)
	}

	if td.Metadata["description"] != "This PR fixes the 30s timeout in auth middleware.\n\nLinked to PROJ-123." {
		t.Errorf("description = %q, want PR body", td.Metadata["description"])
	}
	if td.Metadata["base_branch"] != "main" {
		t.Errorf("base_branch = %q, want 'main'", td.Metadata["base_branch"])
	}
	if td.Metadata["head_branch"] != "feature/auth-fix" {
		t.Errorf("head_branch = %q, want 'feature/auth-fix'", td.Metadata["head_branch"])
	}
	if td.Metadata["labels"] != "bug, security" {
		t.Errorf("labels = %q, want 'bug, security'", td.Metadata["labels"])
	}
	if td.Metadata["draft"] != "" {
		t.Errorf("draft = %q, want empty for non-draft", td.Metadata["draft"])
	}
}

func TestNormalizeGitHub_PRDraft(t *testing.T) {
	payload := map[string]any{
		"action": "opened",
		"pull_request": map[string]any{
			"title": "WIP: auth fix",
			"user":  map[string]any{"login": "kalvis"},
			"head":  map[string]any{"sha": "abc"},
			"base":  map[string]any{"ref": "main"},
			"draft": true,
		},
	}

	td, err := NormalizeGitHubEvent("pull_request.opened", payload)
	if err != nil {
		t.Fatal(err)
	}

	if td.Metadata["draft"] != "true" {
		t.Errorf("draft = %q, want 'true'", td.Metadata["draft"])
	}
}

func TestNormalizeGitHub_IssueMetadata(t *testing.T) {
	payload := map[string]any{
		"action": "opened",
		"issue": map[string]any{
			"number":   float64(10),
			"title":    "Add dark mode",
			"body":     "Users want dark mode support.",
			"html_url": "https://github.com/org/repo/issues/10",
			"user":     map[string]any{"login": "user1"},
			"labels":   []any{map[string]any{"name": "enhancement"}},
		},
	}

	td, err := NormalizeGitHubEvent("issues.opened", payload)
	if err != nil {
		t.Fatal(err)
	}

	if td.Metadata["description"] != "Users want dark mode support." {
		t.Errorf("description = %q, want issue body", td.Metadata["description"])
	}
	if td.Metadata["labels"] != "enhancement" {
		t.Errorf("labels = %q, want 'enhancement'", td.Metadata["labels"])
	}
}

func TestNormalizeGitLab_MRMetadata(t *testing.T) {
	payload := map[string]any{
		"object_kind": "merge_request",
		"user":        map[string]any{"username": "kalvis"},
		"object_attributes": map[string]any{
			"title":         "Add pagination",
			"iid":           float64(7),
			"url":           "https://gitlab.com/org/repo/-/merge_requests/7",
			"description":   "Adds offset/limit params to the users endpoint.",
			"target_branch": "main",
			"source_branch": "feature/pagination",
			"last_commit":   map[string]any{"id": "def456"},
			"labels":        []any{map[string]any{"title": "enhancement"}},
		},
		"project": map[string]any{
			"name":      "repo",
			"namespace": "org",
		},
	}

	td, err := NormalizeGitLabEvent("merge_request", payload)
	if err != nil {
		t.Fatal(err)
	}

	if td.Metadata["description"] != "Adds offset/limit params to the users endpoint." {
		t.Errorf("description = %q, want MR description", td.Metadata["description"])
	}
	if td.Metadata["base_branch"] != "main" {
		t.Errorf("base_branch = %q, want 'main'", td.Metadata["base_branch"])
	}
	if td.Metadata["head_branch"] != "feature/pagination" {
		t.Errorf("head_branch = %q, want 'feature/pagination'", td.Metadata["head_branch"])
	}
	if td.Metadata["labels"] != "enhancement" {
		t.Errorf("labels = %q, want 'enhancement'", td.Metadata["labels"])
	}
}

func TestNormalizeGitLab_IssueMetadata(t *testing.T) {
	payload := map[string]any{
		"object_kind": "issue",
		"user":        map[string]any{"username": "dev"},
		"object_attributes": map[string]any{
			"title":       "Fix crash",
			"iid":         float64(3),
			"url":         "https://gitlab.com/org/repo/-/issues/3",
			"description": "App crashes on startup.",
			"labels":      []any{map[string]any{"title": "bug"}, map[string]any{"title": "critical"}},
		},
		"project": map[string]any{
			"name":      "repo",
			"namespace": "org",
		},
	}

	td, err := NormalizeGitLabEvent("issue", payload)
	if err != nil {
		t.Fatal(err)
	}

	if td.Metadata["description"] != "App crashes on startup." {
		t.Errorf("description = %q, want issue description", td.Metadata["description"])
	}
	if td.Metadata["labels"] != "bug, critical" {
		t.Errorf("labels = %q, want 'bug, critical'", td.Metadata["labels"])
	}
}

func TestFromReader_PreNormalizedWithDescription(t *testing.T) {
	input := `{
		"type": "ticket",
		"source": "jira",
		"intent": "Add dark mode",
		"raw_payload": {
			"key": "PROJ-123",
			"description": "Full ticket description here"
		}
	}`
	td, err := FromReader(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if td.Metadata["description"] != "Full ticket description here" {
		t.Errorf("description = %q, want Jira description from raw_payload", td.Metadata["description"])
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

func TestFromKeyValues_MetadataFields(t *testing.T) {
	td, err := FromKeyValues([]string{
		"type=pull_request",
		"source=github",
		"intent=Fix auth",
		"description=Fixes the timeout bug.",
		"base_branch=main",
		"labels=bug, security",
	})
	if err != nil {
		t.Fatal(err)
	}
	if td.Type != "pull_request" {
		t.Errorf("type = %q, want 'pull_request'", td.Type)
	}
	if td.Metadata["description"] != "Fixes the timeout bug." {
		t.Errorf("description = %q, want 'Fixes the timeout bug.'", td.Metadata["description"])
	}
	if td.Metadata["base_branch"] != "main" {
		t.Errorf("base_branch = %q, want 'main'", td.Metadata["base_branch"])
	}
	if td.Metadata["labels"] != "bug, security" {
		t.Errorf("labels = %q, want 'bug, security'", td.Metadata["labels"])
	}
}

func TestFromReader_InvalidJSON(t *testing.T) {
	_, err := FromReader(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
