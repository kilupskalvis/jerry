// GitHub event normalization.

package trigger

import (
	"fmt"

	"github.com/kilupskalvis/jerry/internal/contextstore"
)

// NormalizeGitHubEvent converts a GitHub webhook payload into TriggerData.
func NormalizeGitHubEvent(eventType string, payload map[string]any) (*contextstore.TriggerData, error) {
	trigger := &contextstore.TriggerData{
		Source:     "github",
		RawPayload: payload,
	}

	switch {
	case hasPrefix(eventType, "issues"):
		return normalizeGitHubIssue(trigger, payload)
	case hasPrefix(eventType, "pull_request"):
		return normalizeGitHubPR(trigger, payload)
	case eventType == "push":
		trigger.Type = "push"
		if headCommit, ok := payload["head_commit"].(map[string]any); ok {
			trigger.Intent, _ = headCommit["message"].(string)
		}
		return trigger, nil
	default:
		trigger.Type = "webhook"
		return trigger, nil
	}
}

func normalizeGitHubIssue(trigger *contextstore.TriggerData, payload map[string]any) (*contextstore.TriggerData, error) {
	trigger.Type = "ticket"

	issue, ok := payload["issue"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("GitHub issue event missing 'issue' field")
	}

	trigger.Intent, _ = issue["title"].(string)

	trigger.RawPayload = map[string]any{
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
		trigger.RawPayload["labels"] = labelNames
	}

	return trigger, nil
}

func normalizeGitHubPR(trigger *contextstore.TriggerData, payload map[string]any) (*contextstore.TriggerData, error) {
	trigger.Type = "pull_request"

	pr, ok := payload["pull_request"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("GitHub PR event missing 'pull_request' field")
	}

	trigger.Intent, _ = pr["title"].(string)

	trigger.RawPayload = map[string]any{
		"pr_number": pr["number"],
		"pr_url":    pr["html_url"],
		"pr_body":   pr["body"],
		"author":    extractNestedString(pr, "user", "login"),
	}

	return trigger, nil
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

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
