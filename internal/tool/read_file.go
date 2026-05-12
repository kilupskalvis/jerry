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

const (
	MaxFileReadSize  = 1 * 1024 * 1024 // 1MB
	DefaultReadLimit = 200
)

// NewReadFileTool creates a read_file tool bound to the given repo root.
func NewReadFileTool(repoRoot string) Tool {
	return NewToolFunc(
		"read_file",
		"Read the contents of a file at the given path. Returns file contents with line numbers prepended. Use offset and limit to read specific ranges of large files.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file (relative to repository root)"
				},
				"offset": {
					"type": "integer",
					"description": "Line number to start reading from (1-based, default: 1)"
				},
				"limit": {
					"type": "integer",
					"description": "Maximum number of lines to read (default: 200)"
				}
			},
			"required": ["path"]
		}`),
		func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}
			if args.Path == "" {
				return "Error: missing required parameter 'path'", nil
			}
			if args.Offset <= 0 {
				args.Offset = 1
			}
			if args.Limit <= 0 {
				args.Limit = DefaultReadLimit
			}

			absPath, pathErr := resolvePath(repoRoot, args.Path)
			if pathErr != "" {
				return pathErr, nil
			}

			info, err := os.Stat(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Sprintf("Error: file not found: %s", args.Path), nil
				}
				return fmt.Sprintf("Error: cannot access '%s': %s", args.Path, err), nil
			}

			if info.IsDir() {
				return fmt.Sprintf("Error: '%s' is a directory", args.Path), nil
			}

			data, err := os.ReadFile(absPath)
			if err != nil {
				return fmt.Sprintf("Error: cannot read '%s': %s", args.Path, err), nil
			}

			if len(data) > MaxFileReadSize {
				data = data[:MaxFileReadSize]
			}

			allLines := strings.Split(string(data), "\n")
			totalLines := len(allLines)

			startIdx := args.Offset - 1
			if startIdx >= totalLines {
				return fmt.Sprintf("Error: offset %d exceeds file length (%d lines)", args.Offset, totalLines), nil
			}

			endIdx := startIdx + args.Limit
			if endIdx > totalLines {
				endIdx = totalLines
			}

			lines := allLines[startIdx:endIdx]

			var b strings.Builder
			for i, line := range lines {
				fmt.Fprintf(&b, "%d: %s\n", startIdx+i+1, line)
			}

			result := b.String()

			if startIdx > 0 || endIdx < totalLines {
				result += fmt.Sprintf("\n[showing lines %d-%d of %d total]", args.Offset, startIdx+len(lines), totalLines)
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
