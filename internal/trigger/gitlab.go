// GitLab event normalization.

package trigger

import (
	"fmt"
	"strings"
)

func extractGitLabLabelNames(labels []any) string {
	var names []string
	for _, l := range labels {
		if lm, ok := l.(map[string]any); ok {
			if title, ok := lm["title"].(string); ok {
				names = append(names, title)
			}
		}
	}
	return strings.Join(names, ", ")
}

// NormalizeGitLabEvent converts a GitLab webhook payload into TriggerData.
func NormalizeGitLabEvent(objectKind string, payload map[string]any) (*TriggerData, error) {
	t := &TriggerData{
		Source:     "gitlab",
		RawPayload: payload,
	}

	extractGitLabAuthor(t, payload)

	switch objectKind {
	case "issue":
		t.Type = "ticket"
		attrs, ok := payload["object_attributes"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("GitLab issue event missing 'object_attributes' field")
		}
		t.Intent, _ = attrs["title"].(string)
		if n, ok := attrs["iid"].(float64); ok {
			t.Number = int(n)
		}
		t.URL, _ = attrs["url"].(string)
		extractGitLabRepo(t, payload)

		t.Metadata = make(map[string]string)
		if desc, ok := attrs["description"].(string); ok && desc != "" {
			t.Metadata["description"] = desc
		}
		if labels, ok := attrs["labels"].([]any); ok {
			if s := extractGitLabLabelNames(labels); s != "" {
				t.Metadata["labels"] = s
			}
		}
	case "merge_request":
		t.Type = "pull_request"
		attrs, ok := payload["object_attributes"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("GitLab merge request event missing 'object_attributes' field")
		}
		t.Intent, _ = attrs["title"].(string)
		if n, ok := attrs["iid"].(float64); ok {
			t.Number = int(n)
		}
		t.URL, _ = attrs["url"].(string)
		if lastCommit, ok := attrs["last_commit"].(map[string]any); ok {
			t.HeadSHA, _ = lastCommit["id"].(string)
		}
		extractGitLabRepo(t, payload)

		t.Metadata = make(map[string]string)
		if desc, ok := attrs["description"].(string); ok && desc != "" {
			t.Metadata["description"] = desc
		}
		if target, ok := attrs["target_branch"].(string); ok && target != "" {
			t.Metadata["base_branch"] = target
		}
		if source, ok := attrs["source_branch"].(string); ok && source != "" {
			t.Metadata["head_branch"] = source
		}
		if labels, ok := attrs["labels"].([]any); ok {
			if s := extractGitLabLabelNames(labels); s != "" {
				t.Metadata["labels"] = s
			}
		}
	case "push":
		t.Type = "push"
		t.HeadSHA, _ = payload["checkout_sha"].(string)
		extractGitLabRepo(t, payload)
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

func extractGitLabAuthor(t *TriggerData, payload map[string]any) {
	if user, ok := payload["user"].(map[string]any); ok {
		if username, ok := user["username"].(string); ok {
			t.Author = username
			return
		}
	}
	if username, ok := payload["user_username"].(string); ok {
		t.Author = username
	}
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
