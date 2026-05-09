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
		}
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

	t.RawPayload = map[string]any{
		"issue_number": issue["number"],
		"issue_url":    issue["html_url"],
		"issue_body":   issue["body"],
		"author":       extractNestedString(issue, "user", "login"),
	}

	if labels, ok := issue["labels"].([]any); ok {
		labelNames := make([]any, 0, len(labels))
		for _, l := range labels {
			if lMap, ok := l.(map[string]any); ok {
				if name, ok := lMap["name"].(string); ok {
					labelNames = append(labelNames, name)
				}
			}
		}
		t.RawPayload["labels"] = labelNames
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

	t.RawPayload = map[string]any{
		"pr_number": pr["number"],
		"pr_url":    pr["html_url"],
		"pr_body":   pr["body"],
		"author":    extractNestedString(pr, "user", "login"),
	}

	return t, nil
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
