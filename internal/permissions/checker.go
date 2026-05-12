package permissions

import "encoding/json"

// Checker evaluates whether a tool call is permitted.
type Checker interface {
	Check(toolName string, input json.RawMessage) *Denial
}

// ResolvedChecker evaluates tool calls against merged permissions.
type ResolvedChecker struct {
	perms  Permissions
	source string
}

// NewChecker creates a checker from resolved permissions.
func NewChecker(perms Permissions, source string) *ResolvedChecker {
	return &ResolvedChecker{perms: perms, source: source}
}

// Check evaluates a tool call. Returns nil if allowed, or a Denial if blocked.
func (c *ResolvedChecker) Check(toolName string, input json.RawMessage) *Denial {
	if c == nil {
		return nil
	}

	matchInput := extractMatchInput(toolName, input)

	for _, pattern := range c.perms.DenyFor(toolName) {
		if MatchGlob(pattern, matchInput) {
			return &Denial{
				Tool:    toolName,
				Input:   matchInput,
				Pattern: pattern,
				Source:  c.source,
			}
		}
	}

	allowPatterns := c.perms.AllowFor(toolName)
	if allowPatterns == nil {
		return nil
	}

	for _, pattern := range allowPatterns {
		if MatchGlob(pattern, matchInput) {
			return nil
		}
	}

	return &Denial{
		Tool:   toolName,
		Input:  matchInput,
		Source: c.source,
	}
}

func extractMatchInput(toolName string, input json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return ""
	}

	switch toolName {
	case "bash":
		if cmd, ok := args["command"].(string); ok {
			return cmd
		}
	case "read_file", "write_file":
		if path, ok := args["path"].(string); ok {
			return path
		}
	}

	return ""
}
