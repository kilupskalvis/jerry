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
				map[string]any{"name": "motif"},
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
	if trigger.RawPayload["issue_number"] != float64(42) {
		t.Errorf("issue_number = %v, want 42", trigger.RawPayload["issue_number"])
	}
}

func TestNormalizeGitHub_Push(t *testing.T) {
	payload := map[string]any{
		"ref": "refs/heads/main",
		"head_commit": map[string]any{
			"message": "update api spec",
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
}

func TestNormalizeGitLab_Issue(t *testing.T) {
	payload := map[string]any{
		"object_kind": "issue",
		"object_attributes": map[string]any{
			"title":       "Fix login bug",
			"description": "The login form crashes on empty email",
			"iid":         float64(15),
			"url":         "https://gitlab.com/org/repo/-/issues/15",
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

func TestFromReader_InvalidJSON(t *testing.T) {
	_, err := FromReader(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
