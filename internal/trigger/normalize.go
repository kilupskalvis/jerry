// Package trigger normalizes external event payloads into Motif's TriggerData format.
package trigger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kilupskalvis/motif/internal/contextstore"
)

// FromFile reads a JSON file and returns TriggerData. The file can contain
// either a raw event payload (auto-detected and normalized) or a pre-normalized
// TriggerData object.
func FromFile(path string) (*contextstore.TriggerData, error) {
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read trigger file: %w", readErr)
	}
	return parse(data)
}

// FromReader reads JSON from a reader (typically stdin) and returns TriggerData.
func FromReader(r io.Reader) (*contextstore.TriggerData, error) {
	data, readErr := io.ReadAll(r)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read trigger data: %w", readErr)
	}
	return parse(data)
}

func parse(data []byte) (*contextstore.TriggerData, error) {
	var raw map[string]any
	if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
		return nil, fmt.Errorf("invalid trigger JSON: %w", unmarshalErr)
	}

	// If it already looks like TriggerData (has "type" and "source"), use it directly.
	if _, hasType := raw["type"]; hasType {
		if _, hasSource := raw["source"]; hasSource {
			var trigger contextstore.TriggerData
			if unmarshalErr := json.Unmarshal(data, &trigger); unmarshalErr != nil {
				return nil, fmt.Errorf("invalid trigger data: %w", unmarshalErr)
			}
			return &trigger, nil
		}
	}

	// Try to auto-detect and normalize the payload.
	if action, ok := raw["action"].(string); ok {
		if _, hasIssue := raw["issue"]; hasIssue {
			return NormalizeGitHubEvent("issues."+action, raw)
		}
		if _, hasPR := raw["pull_request"]; hasPR {
			return NormalizeGitHubEvent("pull_request."+action, raw)
		}
	}

	// GitLab events have "object_kind".
	if objectKind, ok := raw["object_kind"].(string); ok {
		return NormalizeGitLabEvent(objectKind, raw)
	}

	// Unknown format — wrap as generic trigger.
	return &contextstore.TriggerData{
		Type:       "webhook",
		Source:     "unknown",
		Intent:     "",
		RawPayload: raw,
	}, nil
}
