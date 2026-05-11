// Package trigger normalizes external event payloads into Jerry's TriggerData format.
package trigger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// TriggerData holds information about what initiated the workflow run.
type TriggerData struct {
	Type       string         `json:"type"`
	Source     string         `json:"source"`
	Intent     string         `json:"intent,omitempty"`
	Number     int            `json:"number,omitempty"`
	URL        string         `json:"url,omitempty"`
	Author     string         `json:"author,omitempty"`
	HeadSHA    string         `json:"head_sha,omitempty"`
	RepoOwner  string         `json:"repo_owner,omitempty"`
	RepoName   string         `json:"repo_name,omitempty"`
	RawPayload map[string]any `json:"raw_payload,omitempty"`
}

// FromFile reads a JSON file and returns TriggerData.
func FromFile(path string) (*TriggerData, error) {
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read trigger file: %w", readErr)
	}
	return parse(data)
}

// FromReader reads JSON from a reader (typically stdin) and returns TriggerData.
func FromReader(r io.Reader) (*TriggerData, error) {
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read trigger data: %w", readErr)
	}
	return parse(data)
}

func parse(data []byte) (*TriggerData, error) {
	var raw map[string]any
	if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
		return nil, fmt.Errorf("invalid trigger JSON: %w", unmarshalErr)
	}

	if _, hasType := raw["type"]; hasType {
		if _, hasSource := raw["source"]; hasSource {
			var t TriggerData
			if unmarshalErr := json.Unmarshal(data, &t); unmarshalErr != nil {
				return nil, fmt.Errorf("invalid trigger data: %w", unmarshalErr)
			}
			return &t, nil
		}
	}

	if action, ok := raw["action"].(string); ok {
		if _, hasIssue := raw["issue"]; hasIssue {
			return NormalizeGitHubEvent("issues."+action, raw)
		}
		if _, hasPR := raw["pull_request"]; hasPR {
			return NormalizeGitHubEvent("pull_request."+action, raw)
		}
		if cp, hasCP := raw["client_payload"].(map[string]any); hasCP {
			if _, cpHasType := cp["type"]; cpHasType {
				if _, cpHasSource := cp["source"]; cpHasSource {
					cpJSON, marshalErr := json.Marshal(cp)
					if marshalErr == nil {
						var t TriggerData
						if unmarshalErr := json.Unmarshal(cpJSON, &t); unmarshalErr == nil {
							return &t, nil
						}
					}
				}
			}
		}
	}

	if objectKind, ok := raw["object_kind"].(string); ok {
		return NormalizeGitLabEvent(objectKind, raw)
	}

	if _, hasRef := raw["ref"]; hasRef {
		if _, hasHeadCommit := raw["head_commit"]; hasHeadCommit {
			return NormalizeGitHubEvent("push", raw)
		}
	}

	return &TriggerData{
		Type:       "webhook",
		Source:     "unknown",
		Intent:     "",
		RawPayload: raw,
	}, nil
}
