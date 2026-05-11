package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

func NewPostPRCommentTool(triggerRef *trigger.TriggerData, cfg *githubCfg) Tool {
	return NewToolFunc(
		"post_pr_comment",
		"Post a comment on the triggering pull request or issue.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"body": {
					"type": "string",
					"description": "The comment body (supports GitHub-flavored Markdown)"
				}
			},
			"required": ["body"]
		}`),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Body string `json:"body"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}
			if args.Body == "" {
				return "Error: body is required", nil
			}

			gh, err := resolveGitHubContext(triggerRef, cfg)
			if err != nil {
				return fmt.Sprintf("Error: %v", err), nil
			}

			if triggerRef.Number == 0 {
				return "Error: cannot determine PR/issue number from trigger", nil
			}

			url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments",
				gh.BaseURL, gh.Owner, gh.Repo, triggerRef.Number)

			_, apiErr := githubAPI("POST", url, gh.Token, map[string]string{"body": args.Body})
			if apiErr != nil {
				return fmt.Sprintf("Error: %v", apiErr), nil
			}

			return fmt.Sprintf("Comment posted on #%d", triggerRef.Number), nil
		},
	)
}
