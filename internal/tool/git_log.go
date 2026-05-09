package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// MaxGitLogCount is the maximum number of commits returned by git_log.
const MaxGitLogCount = 50

// NewGitLogTool creates a git_log tool bound to the given repo root.
func NewGitLogTool(repoRoot string) Tool {
	return NewToolFunc(
		"git_log",
		"View recent git commits.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"count": {
					"type": "integer",
					"description": "Number of commits to show (default: 10, max: 50)"
				},
				"path": {
					"type": "string",
					"description": "Optional file path to filter commits (relative to repo root)"
				}
			}
		}`),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Count *int   `json:"count"`
				Path  string `json:"path"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}

			count := 10
			if args.Count != nil {
				count = *args.Count
			}
			if count < 1 {
				count = 1
			}
			if count > MaxGitLogCount {
				count = MaxGitLogCount
			}

			if violation := validateGitPath(args.Path, repoRoot); violation != "" {
				return violation, nil
			}

			gitArgs := []string{"log", "--oneline", "--no-decorate", "-n", fmt.Sprintf("%d", count)}
			if args.Path != "" {
				gitArgs = append(gitArgs, "--", args.Path)
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
	)
}
