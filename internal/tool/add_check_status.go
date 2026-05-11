package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

func NewAddCheckStatusTool(triggerRef *trigger.TriggerData, cfg *githubCfg) Tool {
	return NewToolFunc(
		"add_check_status",
		"Report a status check result (pass/fail) on the current commit.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Name of the check (e.g., 'Jerry Security Scan')"
				},
				"status": {
					"type": "string",
					"enum": ["success", "failure"],
					"description": "Check result: success or failure"
				},
				"summary": {
					"type": "string",
					"description": "Summary of the check results"
				}
			},
			"required": ["name", "status", "summary"]
		}`),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Summary string `json:"summary"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}
			if args.Name == "" || args.Status == "" || args.Summary == "" {
				return "Error: name, status, and summary are all required", nil
			}
			if args.Status != "success" && args.Status != "failure" {
				return "Error: status must be 'success' or 'failure'", nil
			}

			gh, err := resolveGitHubContext(triggerRef, cfg)
			if err != nil {
				return fmt.Sprintf("Error: %v", err), nil
			}

			if triggerRef.HeadSHA == "" {
				return "Error: cannot determine commit SHA from trigger", nil
			}

			conclusion := args.Status
			url := fmt.Sprintf("%s/repos/%s/%s/check-runs",
				gh.BaseURL, gh.Owner, gh.Repo)

			payload := map[string]any{
				"name":       args.Name,
				"head_sha":   triggerRef.HeadSHA,
				"status":     "completed",
				"conclusion": conclusion,
				"output": map[string]string{
					"title":   args.Name,
					"summary": args.Summary,
				},
			}

			_, apiErr := githubAPI("POST", url, gh.Token, payload)
			if apiErr != nil {
				return fmt.Sprintf("Error: %v", apiErr), nil
			}

			return fmt.Sprintf("Check '%s' reported as %s", args.Name, args.Status), nil
		},
	)
}
