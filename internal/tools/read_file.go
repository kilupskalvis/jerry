// read_file tool: reads a file's contents with line numbers.

package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kilupskalvis/motif/internal/llm"
)

// MaxFileReadSize is the maximum bytes read from a single file.
// Larger files are truncated with a note.
const MaxFileReadSize = 1 * 1024 * 1024 // 1MB

// NewReadFileTool creates a read_file tool bound to the given repo root.
func NewReadFileTool(repoRoot string) Tool {
	return Tool{
		Definition: llm.ToolDef{
			Name:        "read_file",
			Description: "Read the contents of a file at the given path. Returns file contents with line numbers prepended.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file (relative to repository root)",
					},
				},
				"required": []any{"path"},
			},
		},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "Error: missing required parameter 'path'", nil
			}

			absPath, pathErr := resolvePath(repoRoot, path)
			if pathErr != "" {
				return pathErr, nil
			}

			info, err := os.Stat(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Sprintf("Error: file not found: %s", path), nil
				}
				return fmt.Sprintf("Error: cannot access '%s': %s", path, err), nil
			}

			if info.IsDir() {
				return fmt.Sprintf("Error: '%s' is a directory, use list_directory instead", path), nil
			}

			data, err := os.ReadFile(absPath)
			if err != nil {
				return fmt.Sprintf("Error: cannot read '%s': %s", path, err), nil
			}

			truncated := false
			if len(data) > MaxFileReadSize {
				data = data[:MaxFileReadSize]
				truncated = true
			}

			lines := strings.Split(string(data), "\n")
			var b strings.Builder
			for i, line := range lines {
				fmt.Fprintf(&b, "%d: %s\n", i+1, line)
			}

			result := b.String()
			if truncated {
				result += "\n[truncated — file exceeds 1MB, showing first 1MB]"
			}

			return result, nil
		},
	}
}

// resolvePath resolves a relative path against the repo root and validates
// it does not escape the root. Returns the absolute path and an empty error
// string on success, or an empty path and error message on failure.
func resolvePath(repoRoot, path string) (string, string) {
	absPath := filepath.Join(repoRoot, path)
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Sprintf("Error: invalid path '%s': %s", path, err)
	}

	// Ensure the resolved path is within the repo root.
	// Use filepath.Rel to handle symlinks and edge cases.
	absRoot, _ := filepath.Abs(repoRoot)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return "", fmt.Sprintf("Error: path '%s' escapes repository root", path)
	}

	return absPath, ""
}
