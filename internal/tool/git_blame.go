package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// NewGitBlameTool creates a git_blame tool bound to the given repo root.
func NewGitBlameTool(repoRoot string) Tool {
	return NewToolFunc(
		"git_blame",
		"View git blame for a file, showing who last modified each line.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "File path (relative to repo root)"
				},
				"start_line": {
					"type": "integer",
					"description": "Optional start line number"
				},
				"end_line": {
					"type": "integer",
					"description": "Optional end line number"
				}
			},
			"required": ["path"]
		}`),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Path      string `json:"path"`
				StartLine *int   `json:"start_line"`
				EndLine   *int   `json:"end_line"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}

			if args.Path == "" {
				return "Error: missing required parameter 'path'", nil
			}

			if violation := validateGitPath(args.Path, repoRoot); violation != "" {
				return violation, nil
			}

			gitArgs := []string{"blame"}

			if args.StartLine != nil && args.EndLine != nil {
				gitArgs = append(gitArgs, "-L", fmt.Sprintf("%d,%d", *args.StartLine, *args.EndLine))
			} else if args.StartLine != nil {
				gitArgs = append(gitArgs, "-L", fmt.Sprintf("%d,", *args.StartLine))
			}

			gitArgs = append(gitArgs, args.Path)

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
	)
}
