// read_file tool: reads a file's contents with line numbers.

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxFileReadSize is the maximum bytes read from a single file.
const MaxFileReadSize = 1 * 1024 * 1024 // 1MB

// NewReadFileTool creates a read_file tool bound to the given repo root.
func NewReadFileTool(repoRoot string) Tool {
	return NewToolFunc(
		"read_file",
		"Read the contents of a file at the given path. Returns file contents with line numbers prepended.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file (relative to repository root)"
				}
			},
			"required": ["path"]
		}`),
		func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}
			if args.Path == "" {
				return "Error: missing required parameter 'path'", nil
			}

			absPath, pathErr := resolvePath(repoRoot, args.Path)
			if pathErr != "" {
				return pathErr, nil
			}

			if IsSensitivePath(args.Path) {
				return fmt.Sprintf("Error: access denied — '%s' is a sensitive file (may contain secrets)", args.Path), nil
			}

			info, err := os.Stat(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Sprintf("Error: file not found: %s", args.Path), nil
				}
				return fmt.Sprintf("Error: cannot access '%s': %s", args.Path, err), nil
			}

			if info.IsDir() {
				return fmt.Sprintf("Error: '%s' is a directory, use list_directory instead", args.Path), nil
			}

			data, err := os.ReadFile(absPath)
			if err != nil {
				return fmt.Sprintf("Error: cannot read '%s': %s", args.Path, err), nil
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
	)
}

// resolvePath resolves a relative path against the repo root and validates
// it does not escape the root.
func resolvePath(repoRoot, path string) (string, string) {
	absPath := filepath.Join(repoRoot, path)
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Sprintf("Error: invalid path '%s': %s", path, err)
	}

	absRoot, _ := filepath.Abs(repoRoot)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return "", fmt.Sprintf("Error: path '%s' escapes repository root", path)
	}

	return absPath, ""
}
