package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// NewWriteFileTool creates a write_file tool bound to the given repo root.
func NewWriteFileTool(repoRoot string) Tool {
	return NewToolFunc(
		"write_file",
		"Write content to a file, creating it if it doesn't exist or overwriting if it does. Parent directories are created automatically.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file (relative to repository root)"
				},
				"content": {
					"type": "string",
					"description": "Content to write to the file"
				}
			},
			"required": ["path", "content"]
		}`),
		func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Path    string `json:"path"`
				Content string `json:"content"`
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

			// Create parent directories.
			dir := filepath.Dir(absPath)
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return fmt.Sprintf("Error: cannot create directory '%s': %s", dir, mkErr), nil
			}

			// Atomic write: write to temp file, then rename.
			tmp := absPath + ".jerry.tmp"
			if writeErr := os.WriteFile(tmp, []byte(args.Content), 0o644); writeErr != nil {
				return fmt.Sprintf("Error: cannot write to '%s': %s", args.Path, writeErr), nil
			}

			if renameErr := os.Rename(tmp, absPath); renameErr != nil {
				_ = os.Remove(tmp)
				return fmt.Sprintf("Error: cannot write to '%s': %s", args.Path, renameErr), nil
			}

			return fmt.Sprintf("Successfully wrote %d bytes to %s", len(args.Content), args.Path), nil
		},
	)
}
