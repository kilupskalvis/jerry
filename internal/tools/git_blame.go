package tools

import (
	"context"
	"fmt"

	"github.com/kilupskalvis/jerry/internal/llm"
)

// NewGitBlameTool creates a git_blame tool bound to the given repo root.
func NewGitBlameTool(repoRoot string) Tool {
	return Tool{
		Definition: llm.ToolDef{
			Name:        "git_blame",
			Description: "View git blame for a file, showing who last modified each line.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "File path (relative to repo root)",
					},
					"start_line": map[string]any{
						"type":        "integer",
						"description": "Optional start line number",
					},
					"end_line": map[string]any{
						"type":        "integer",
						"description": "Optional end line number",
					},
				},
				"required": []any{"path"},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "Error: missing required parameter 'path'", nil
			}

			if violation := validateGitPath(path, repoRoot); violation != "" {
				return violation, nil
			}

			gitArgs := []string{"blame"}

			startLine, hasStart := args["start_line"].(float64)
			endLine, hasEnd := args["end_line"].(float64)
			if hasStart && hasEnd {
				gitArgs = append(gitArgs, "-L", fmt.Sprintf("%d,%d", int(startLine), int(endLine)))
			} else if hasStart {
				gitArgs = append(gitArgs, "-L", fmt.Sprintf("%d,", int(startLine)))
			}

			gitArgs = append(gitArgs, path)

			result, gitErr := runGit(ctx, repoRoot, gitArgs...)
			if gitErr != nil {
				return fmt.Sprintf("Error: %s\n%s", gitErr, result), nil
			}

			if len(result) > MaxDiffOutputSize {
				totalKB := len(result) / 1024
				result = result[:MaxDiffOutputSize]
				result += fmt.Sprintf("\n\n[blame output truncated — showing first 50KB of %dKB]", totalKB)
			}

			return result, nil
		},
	}
}
