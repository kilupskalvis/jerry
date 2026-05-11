package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

func NewCreatePullRequestTool(repoRoot string, triggerRef *trigger.TriggerData, cfg *githubCfg) Tool {
	return NewToolFunc(
		"create_pull_request",
		"Create a git branch, commit all changes, push, and open a pull request on GitHub. Returns the PR URL.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {
					"type": "string",
					"description": "Pull request title"
				},
				"body": {
					"type": "string",
					"description": "Pull request description (supports Markdown)"
				},
				"branch": {
					"type": "string",
					"description": "Branch name (default: jerry/<run-id-fragment>)"
				}
			},
			"required": ["title"]
		}`),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Title  string `json:"title"`
				Body   string `json:"body"`
				Branch string `json:"branch"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}
			if args.Title == "" {
				return "Error: title is required", nil
			}

			gh, err := resolveGitHubContext(triggerRef, cfg)
			if err != nil {
				return fmt.Sprintf("Error: %v", err), nil
			}

			branch := args.Branch
			if branch == "" {
				branch = "jerry/" + sanitizeBranch(args.Title)
			}

			baseBranch, gitErr := gitCmd(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
			if gitErr != nil {
				return fmt.Sprintf("Error: cannot determine current branch: %v", gitErr), nil
			}

			if _, err := gitCmd(repoRoot, "checkout", "-b", branch); err != nil {
				return fmt.Sprintf("Error: cannot create branch %q: %v", branch, err), nil
			}

			if _, err := gitCmd(repoRoot, "add", "-A"); err != nil {
				return fmt.Sprintf("Error: git add failed: %v", err), nil
			}

			status, _ := gitCmd(repoRoot, "status", "--porcelain")
			if strings.TrimSpace(status) == "" {
				_ = gitCmdNoOutput(repoRoot, "checkout", baseBranch)
				_ = gitCmdNoOutput(repoRoot, "branch", "-D", branch)
				return "Error: no changes to commit", nil
			}

			if _, err := gitCmd(repoRoot, "commit", "-m", args.Title); err != nil {
				return fmt.Sprintf("Error: git commit failed: %v", err), nil
			}

			pushURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git",
				gh.Token, gh.Owner, gh.Repo)
			if _, err := gitCmd(repoRoot, "push", pushURL, branch); err != nil {
				return fmt.Sprintf("Error: git push failed: %v", err), nil
			}

			url := fmt.Sprintf("%s/repos/%s/%s/pulls",
				gh.BaseURL, gh.Owner, gh.Repo)

			payload := map[string]string{
				"title": args.Title,
				"head":  branch,
				"base":  baseBranch,
			}
			if args.Body != "" {
				payload["body"] = args.Body
			}

			respBody, apiErr := githubAPI("POST", url, gh.Token, payload)
			if apiErr != nil {
				return fmt.Sprintf("Error: %v", apiErr), nil
			}

			var pr struct {
				HTMLURL string `json:"html_url"`
				Number  int    `json:"number"`
			}
			if unmarshalErr := json.Unmarshal([]byte(respBody), &pr); unmarshalErr != nil {
				return fmt.Sprintf("PR created but could not parse response: %s", respBody), nil //nolint:nilerr // tool errors are returned as strings, not Go errors
			}

			return fmt.Sprintf("Pull request #%d created: %s", pr.Number, pr.HTMLURL), nil
		},
	)
}

func gitCmd(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func gitCmdNoOutput(repoRoot string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	return cmd.Run()
}

func sanitizeBranch(title string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", ":", "", "'", "", "\"", "")
	branch := r.Replace(strings.ToLower(title))
	if len(branch) > 50 {
		branch = branch[:50]
	}
	return strings.TrimRight(branch, "-")
}
