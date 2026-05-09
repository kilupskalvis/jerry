package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// MaxDiffOutputSize is the maximum bytes returned by git_diff/git_blame.
const MaxDiffOutputSize = 50 * 1024 // 50KB

// NewGitDiffTool creates a git_diff tool bound to the given repo root.
func NewGitDiffTool(repoRoot string) Tool {
	return NewToolFunc(
		"git_diff",
		"View git diff of changes.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"ref": {
					"type": "string",
					"description": "Git ref to diff against (e.g., 'HEAD', 'main', 'HEAD~3'). Defaults to unstaged changes."
				},
				"path": {
					"type": "string",
					"description": "Optional file path to filter diff (relative to repo root)"
				}
			}
		}`),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Ref  string `json:"ref"`
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}

			if violation := validateGitPath(args.Path, repoRoot); violation != "" {
				return violation, nil
			}

			gitArgs := []string{"diff"}
			if args.Ref != "" {
				gitArgs = append(gitArgs, args.Ref)
			}
			if args.Path != "" {
				gitArgs = append(gitArgs, "--", args.Path)
			}

			result, gitErr := runGit(ctx, repoRoot, gitArgs...)
			if gitErr != nil {
				return fmt.Sprintf("Error: %s\n%s", gitErr, result), nil
			}
			if result == "" {
				return "No changes found", nil
			}

			if len(result) > MaxDiffOutputSize {
				totalKB := len(result) / 1024
				result = result[:MaxDiffOutputSize]
				result += fmt.Sprintf("\n\n[diff truncated — showing first 50KB of %dKB]", totalKB)
			}

			return result, nil
		},
	)
}
