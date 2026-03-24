// Sensitive file detection: blocks agent access to files containing secrets.

package tools

import (
	"path/filepath"
	"strings"
)

// sensitivePatterns lists file name patterns that agents must not read.
// These files commonly contain secrets (API keys, tokens, passwords).
var sensitivePatterns = []string{
	".env",
	".env.*",
}

// IsSensitivePath reports whether the given path (relative to repo root)
// matches a sensitive file pattern. Agents are blocked from reading these.
func IsSensitivePath(relPath string) bool {
	base := filepath.Base(relPath)
	for _, pattern := range sensitivePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	// Also block paths inside directories that are sensitive.
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, part := range parts {
		for _, pattern := range sensitivePatterns {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}
	return false
}
