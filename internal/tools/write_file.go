package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kilupskalvis/jerry/internal/llm"
)

// NewWriteFileTool creates a write_file tool bound to the given repo root.
func NewWriteFileTool(repoRoot string) Tool {
	return Tool{
		Definition: llm.ToolDef{
			Name:        "write_file",
			Description: "Write content to a file, creating it if it doesn't exist or overwriting if it does. Parent directories are created automatically.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file (relative to repository root)",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []any{"path", "content"},
			},
		},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)

			if path == "" {
				return "Error: missing required parameter 'path'", nil
			}

			absPath, pathErr := resolvePath(repoRoot, path)
			if pathErr != "" {
				return pathErr, nil
			}

			if IsSensitivePath(path) {
				return fmt.Sprintf("Error: access denied — '%s' is a sensitive file (may contain secrets)", path), nil
			}

			// Create parent directories.
			dir := filepath.Dir(absPath)
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return fmt.Sprintf("Error: cannot create directory '%s': %s", dir, mkErr), nil
			}

			// Atomic write: write to temp file, then rename.
			tmp := absPath + ".jerry.tmp"
			if writeErr := os.WriteFile(tmp, []byte(content), 0o644); writeErr != nil {
				return fmt.Sprintf("Error: cannot write to '%s': %s", path, writeErr), nil
			}

			if renameErr := os.Rename(tmp, absPath); renameErr != nil {
				_ = os.Remove(tmp)
				return fmt.Sprintf("Error: cannot write to '%s': %s", path, renameErr), nil
			}

			return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
		},
	}
}
