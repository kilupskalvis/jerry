package tools

import (
	"context"
	"fmt"

	"github.com/kilupskalvis/jerry/internal/llm"
)

// MaxGitLogCount is the maximum number of commits returned by git_log.
const MaxGitLogCount = 50

// NewGitLogTool creates a git_log tool bound to the given repo root.
func NewGitLogTool(repoRoot string) Tool {
	return Tool{
		Definition: llm.ToolDef{
			Name:        "git_log",
			Description: "View recent git commits.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"count": map[string]any{
						"type":        "integer",
						"description": "Number of commits to show (default: 10, max: 50)",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Optional file path to filter commits (relative to repo root)",
					},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			count := 10
			if c, ok := args["count"].(float64); ok {
				count = int(c)
			}
			if count < 1 {
				count = 1
			}
			if count > MaxGitLogCount {
				count = MaxGitLogCount
			}

			path, _ := args["path"].(string)
			if violation := validateGitPath(path, repoRoot); violation != "" {
				return violation, nil
			}

			gitArgs := []string{"log", "--oneline", "--no-decorate", "-n", fmt.Sprintf("%d", count)}
			if path != "" {
				gitArgs = append(gitArgs, "--", path)
			}

			result, gitErr := runGit(ctx, repoRoot, gitArgs...)
			if gitErr != nil {
				return fmt.Sprintf("Error: %s\n%s", gitErr, result), nil
			}
			if result == "" {
				return "No commits found", nil
			}
			return result, nil
		},
	}
}
