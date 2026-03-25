// GitLab event normalization.

package trigger

import (
	"github.com/kilupskalvis/jerry/internal/contextstore"
)

// NormalizeGitLabEvent converts a GitLab webhook payload into TriggerData.
func NormalizeGitLabEvent(objectKind string, payload map[string]any) (*contextstore.TriggerData, error) {
	trigger := &contextstore.TriggerData{
		Source:     "gitlab",
		RawPayload: payload,
	}

	switch objectKind {
	case "issue":
		trigger.Type = "ticket"
		if attrs, ok := payload["object_attributes"].(map[string]any); ok {
			trigger.Intent, _ = attrs["title"].(string)
			trigger.RawPayload = map[string]any{
				"issue_id":   attrs["iid"],
				"issue_url":  attrs["url"],
				"issue_body": attrs["description"],
			}
		}
	case "merge_request":
		trigger.Type = "pull_request"
		if attrs, ok := payload["object_attributes"].(map[string]any); ok {
			trigger.Intent, _ = attrs["title"].(string)
			trigger.RawPayload = map[string]any{
				"mr_id":   attrs["iid"],
				"mr_url":  attrs["url"],
				"mr_body": attrs["description"],
			}
		}
	case "push":
		trigger.Type = "push"
		if commits, ok := payload["commits"].([]any); ok && len(commits) > 0 {
			if last, ok := commits[len(commits)-1].(map[string]any); ok {
				trigger.Intent, _ = last["message"].(string)
			}
		}
	default:
		trigger.Type = "webhook"
	}

	return trigger, nil
}
