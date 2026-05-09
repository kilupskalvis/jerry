// GitLab event normalization.

package trigger

// NormalizeGitLabEvent converts a GitLab webhook payload into TriggerData.
func NormalizeGitLabEvent(objectKind string, payload map[string]any) (*TriggerData, error) {
	t := &TriggerData{
		Source:     "gitlab",
		RawPayload: payload,
	}

	switch objectKind {
	case "issue":
		t.Type = "ticket"
		if attrs, ok := payload["object_attributes"].(map[string]any); ok {
			t.Intent, _ = attrs["title"].(string)
			t.RawPayload = map[string]any{
				"issue_id":   attrs["iid"],
				"issue_url":  attrs["url"],
				"issue_body": attrs["description"],
			}
		}
	case "merge_request":
		t.Type = "pull_request"
		if attrs, ok := payload["object_attributes"].(map[string]any); ok {
			t.Intent, _ = attrs["title"].(string)
			t.RawPayload = map[string]any{
				"mr_id":   attrs["iid"],
				"mr_url":  attrs["url"],
				"mr_body": attrs["description"],
			}
		}
	case "push":
		t.Type = "push"
		if commits, ok := payload["commits"].([]any); ok && len(commits) > 0 {
			if last, ok := commits[len(commits)-1].(map[string]any); ok {
				t.Intent, _ = last["message"].(string)
			}
		}
	default:
		t.Type = "webhook"
	}

	return t, nil
}
