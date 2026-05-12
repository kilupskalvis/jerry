package permissions

import (
	"path/filepath"
	"strings"
)

// MatchGlob matches input against a glob pattern.
// Supports * (single segment) and ** (recursive/any depth).
func MatchGlob(pattern, input string) bool {
	if pattern == "" || input == "" {
		return false
	}

	if pattern == "**" {
		return true
	}

	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
		return strings.HasPrefix(input, prefix+"/")
	}

	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, input)
	}

	// For path-like patterns (no spaces), use filepath.Match which handles
	// *, ?, and character classes correctly for file paths.
	if !strings.Contains(pattern, " ") && !strings.Contains(input, " ") {
		matched, _ := filepath.Match(pattern, input)
		return matched
	}

	// For command-like patterns (contain spaces), use simple wildcard matching.
	return matchWildcard(pattern, input)
}

func matchWildcard(pattern, input string) bool {
	// Split pattern on * — each non-empty segment must appear in order.
	// First segment must be a prefix. Last segment must be a suffix.
	parts := strings.Split(pattern, "*")

	if len(parts) == 1 {
		return pattern == input
	}

	remaining := input

	for i, part := range parts {
		if part == "" {
			continue
		}

		if i == 0 {
			if !strings.HasPrefix(remaining, part) {
				return false
			}
			remaining = remaining[len(part):]
			continue
		}

		if i == len(parts)-1 {
			return strings.HasSuffix(remaining, part)
		}

		idx := strings.Index(remaining, part)
		if idx == -1 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}

	return true
}

func matchDoubleStar(pattern, input string) bool {
	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := parts[1]

	if prefix != "" && !strings.HasPrefix(input, prefix) {
		return false
	}
	if suffix != "" && !strings.HasSuffix(input, suffix) {
		return false
	}
	return true
}
