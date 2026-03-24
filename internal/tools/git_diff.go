// git_diff tool: view git diff of changes.

package tools

import (
	"context"
	"fmt"

	"github.com/kilupskalvis/motif/internal/llm"
)

// MaxDiffOutputSize is the maximum bytes returned by git_diff/git_blame.
const MaxDiffOutputSize = 50 * 1024 // 50KB

// NewGitDiffTool creates a git_diff tool bound to the given repo root.
func NewGitDiffTool(repoRoot string) Tool {
	return Tool{
		Definition: llm.ToolDef{
			Name:        "git_diff",
			Description: "View git diff of changes.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ref": map[string]any{
						"type":        "string",
						"description": "Git ref to diff against (e.g., 'HEAD', 'main', 'HEAD~3'). Defaults to unstaged changes.",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Optional file path to filter diff (relative to repo root)",
					},
				},
			},
		},
		Execute: func(toolCtx context.Context, args map[string]any) (string, error) {
			ref, _ := args["ref"].(string)
			path, _ := args["path"].(string)

			if violation := validateGitPath(path, repoRoot); violation != "" {
				return violation, nil
			}

			gitArgs := []string{"diff"}
			if ref != "" {
				gitArgs = append(gitArgs, ref)
			}
			if path != "" {
				gitArgs = append(gitArgs, "--", path)
			}

			result, gitErr := runGit(toolCtx, repoRoot, gitArgs...)
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
	}
}
