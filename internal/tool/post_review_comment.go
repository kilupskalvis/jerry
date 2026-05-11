package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

func NewPostReviewCommentTool(triggerRef *trigger.TriggerData, cfg *githubCfg) Tool {
	return NewToolFunc(
		"post_review_comment",
		"Post an inline review comment on a specific file and line in a pull request.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The relative file path to comment on"
				},
				"line": {
					"type": "integer",
					"description": "The line number in the diff to attach the comment to"
				},
				"body": {
					"type": "string",
					"description": "The comment body (supports GitHub-flavored Markdown)"
				}
			},
			"required": ["path", "line", "body"]
		}`),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Path string `json:"path"`
				Line int    `json:"line"`
				Body string `json:"body"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}
			if args.Path == "" || args.Body == "" || args.Line == 0 {
				return "Error: path, line, and body are all required", nil
			}

			gh, err := resolveGitHubContext(triggerRef, cfg)
			if err != nil {
				return fmt.Sprintf("Error: %v", err), nil
			}

			if triggerRef.Number == 0 {
				return "Error: cannot determine PR number from trigger", nil
			}
			if triggerRef.HeadSHA == "" {
				return "Error: cannot determine commit SHA from trigger", nil
			}

			url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/comments",
				gh.BaseURL, gh.Owner, gh.Repo, triggerRef.Number)

			payload := map[string]any{
				"body":      args.Body,
				"commit_id": triggerRef.HeadSHA,
				"path":      args.Path,
				"line":      args.Line,
			}

			_, apiErr := githubAPI("POST", url, gh.Token, payload)
			if apiErr != nil {
				return fmt.Sprintf("Error: %v", apiErr), nil
			}

			return fmt.Sprintf("Review comment posted on %s:%d", args.Path, args.Line), nil
		},
	)
}
