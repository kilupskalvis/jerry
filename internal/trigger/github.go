// GitHub event normalization.

package trigger

import (
	"fmt"
	"strings"
)

// NormalizeGitHubEvent converts a GitHub webhook payload into TriggerData.
func NormalizeGitHubEvent(eventType string, payload map[string]any) (*TriggerData, error) {
	t := &TriggerData{
		Source:     "github",
		RawPayload: payload,
	}

	switch {
	case strings.HasPrefix(eventType, "issues"):
		return normalizeGitHubIssue(t, payload)
	case strings.HasPrefix(eventType, "pull_request"):
		return normalizeGitHubPR(t, payload)
	case eventType == "push":
		t.Type = "push"
		if headCommit, ok := payload["head_commit"].(map[string]any); ok {
			t.Intent, _ = headCommit["message"].(string)
			t.HeadSHA, _ = headCommit["id"].(string)
		}
		t.Author = extractNestedString(payload, "sender", "login")
		extractGitHubRepo(t, payload)
		return t, nil
	default:
		t.Type = "webhook"
		return t, nil
	}
}

func normalizeGitHubIssue(t *TriggerData, payload map[string]any) (*TriggerData, error) {
	t.Type = "ticket"

	issue, ok := payload["issue"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("GitHub issue event missing 'issue' field")
	}

	t.Intent, _ = issue["title"].(string)
	if n, ok := issue["number"].(float64); ok {
		t.Number = int(n)
	}
	t.URL, _ = issue["html_url"].(string)
	t.Author = extractNestedString(issue, "user", "login")
	extractGitHubRepo(t, payload)

	return t, nil
}

func normalizeGitHubPR(t *TriggerData, payload map[string]any) (*TriggerData, error) {
	t.Type = "pull_request"

	pr, ok := payload["pull_request"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("GitHub PR event missing 'pull_request' field")
	}

	t.Intent, _ = pr["title"].(string)
	if n, ok := pr["number"].(float64); ok {
		t.Number = int(n)
	}
	t.URL, _ = pr["html_url"].(string)
	t.Author = extractNestedString(pr, "user", "login")
	t.HeadSHA = extractNestedString(pr, "head", "sha")
	extractGitHubRepo(t, payload)

	return t, nil
}

func extractGitHubRepo(t *TriggerData, payload map[string]any) {
	if repo, ok := payload["repository"].(map[string]any); ok {
		if owner, ok := repo["owner"].(map[string]any); ok {
			t.RepoOwner, _ = owner["login"].(string)
		}
		t.RepoName, _ = repo["name"].(string)
	}
}

func extractNestedString(m map[string]any, keys ...string) string {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			s, _ := current[key].(string)
			return s
		}
		next, ok := current[key].(map[string]any)
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}
