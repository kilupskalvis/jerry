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
			if n, ok := attrs["iid"].(float64); ok {
				t.Number = int(n)
			}
			t.URL, _ = attrs["url"].(string)
		}
		extractGitLabRepo(t, payload)
	case "merge_request":
		t.Type = "pull_request"
		if attrs, ok := payload["object_attributes"].(map[string]any); ok {
			t.Intent, _ = attrs["title"].(string)
			if n, ok := attrs["iid"].(float64); ok {
				t.Number = int(n)
			}
			t.URL, _ = attrs["url"].(string)
			if lastCommit, ok := attrs["last_commit"].(map[string]any); ok {
				t.HeadSHA, _ = lastCommit["id"].(string)
			}
		}
		extractGitLabRepo(t, payload)
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

func extractGitLabRepo(t *TriggerData, payload map[string]any) {
	if project, ok := payload["project"].(map[string]any); ok {
		if namespace, ok := project["namespace"].(string); ok {
			t.RepoOwner = namespace
		}
		if name, ok := project["name"].(string); ok {
			t.RepoName = name
		}
	}
}
