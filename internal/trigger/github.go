// GitHub event normalization.

package trigger

import (
	"fmt"
	"strings"
)

func extractLabelNames(labels []any) string {
	var names []string
	for _, l := range labels {
		if lm, ok := l.(map[string]any); ok {
			if name, ok := lm["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return strings.Join(names, ", ")
}

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

	t.Metadata = make(map[string]string)
	if body, ok := issue["body"].(string); ok && body != "" {
		t.Metadata["description"] = body
	}
	if labels, ok := issue["labels"].([]any); ok {
		if s := extractLabelNames(labels); s != "" {
			t.Metadata["labels"] = s
		}
	}

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

	t.Metadata = make(map[string]string)
	if body, ok := pr["body"].(string); ok && body != "" {
		t.Metadata["description"] = body
	}
	if baseRef := extractNestedString(pr, "base", "ref"); baseRef != "" {
		t.Metadata["base_branch"] = baseRef
	}
	if headRef := extractNestedString(pr, "head", "ref"); headRef != "" {
		t.Metadata["head_branch"] = headRef
	}
	if labels, ok := pr["labels"].([]any); ok {
		if s := extractLabelNames(labels); s != "" {
			t.Metadata["labels"] = s
		}
	}
	if draft, ok := pr["draft"].(bool); ok && draft {
		t.Metadata["draft"] = "true"
	}

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
