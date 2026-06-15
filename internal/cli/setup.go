// jerry setup: generates integration config for external trigger platforms.

package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/spec"
)

func newSetupCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup <platform>",
		Short: "Generate integration config for a trigger platform",
		Long:  "Interactive wizard that generates copy-paste-ready integration config.",
	}

	cmd.AddCommand(newSetupJiraCmd(app))

	return cmd
}

func newSetupJiraCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "jira",
		Short: "Generate Jira integration config",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSetupJira(app)
		},
	}
}

func runSetupJira(app *App) error {
	scanner := bufio.NewScanner(os.Stdin)
	isTTY := isTerminal()

	repoOwner, repoName, repoHost := detectRepo()
	ciPlatform := detectCIPlatform()

	workflows := listSetupWorkflows(app)
	if len(workflows) == 0 {
		return errors.New(errors.CodeJerryDirNotFound,
			"no workflows found in .jerry/ — run 'jerry init --template feature' first")
	}

	workflow := "feature"
	if !contains(workflows, "feature") {
		workflow = workflows[0]
	}

	assignee := "Jerry"

	if isTTY {
		if repoOwner == "" || repoName == "" {
			fmt.Fprintf(os.Stderr, "Repository (owner/name): ")
			if scanner.Scan() {
				parts := strings.SplitN(scanner.Text(), "/", 2)
				if len(parts) == 2 {
					repoOwner, repoName = parts[0], parts[1]
				}
			}
		}

		if ciPlatform == "" {
			fmt.Fprintf(os.Stderr, "CI platform (github/gitlab): ")
			if scanner.Scan() {
				ciPlatform = strings.TrimSpace(scanner.Text())
			}
		}

		if len(workflows) > 1 {
			fmt.Fprintf(os.Stderr, "Workflow to trigger (default: %s): ", workflow)
			if scanner.Scan() {
				if input := strings.TrimSpace(scanner.Text()); input != "" {
					workflow = input
				}
			}
		}

		fmt.Fprintf(os.Stderr, "Jira assignee name to trigger Jerry (default: %s): ", assignee)
		if scanner.Scan() {
			if input := strings.TrimSpace(scanner.Text()); input != "" {
				assignee = input
			}
		}
	}

	if repoOwner == "" || repoName == "" {
		return fmt.Errorf("could not detect repository — run from a git repo or enter owner/name")
	}
	if ciPlatform == "" {
		return fmt.Errorf("could not detect CI platform — use jerry setup jira from a repo with .github/ or .gitlab-ci.yml")
	}

	switch ciPlatform {
	case "github":
		printJiraGitHub(repoOwner, repoName, workflow, assignee)
	case "gitlab":
		var projectID string
		if isTTY {
			fmt.Fprintf(os.Stderr, "GitLab project ID (found in Project Settings → General): ")
			if scanner.Scan() {
				projectID = strings.TrimSpace(scanner.Text())
			}
		}
		if projectID == "" {
			projectID = "<project-id>"
		}
		printJiraGitLab(repoOwner, repoName, repoHost, projectID, workflow, assignee)
	default:
		return fmt.Errorf("unsupported CI platform %q (use github or gitlab)", ciPlatform)
	}

	return nil
}

func printJiraGitHub(owner, repo, workflow, assignee string) {
	fmt.Printf(`Jerry Setup — Jira → GitHub Actions

Repository: %s/%s
Workflow:   %s
Trigger:    Assign ticket to %q

─── Step 1: Create a GitHub Personal Access Token ───

Go to: https://github.com/settings/tokens
Generate a classic token with 'repo' scope.
Save it — you'll need it for the Jira automation rule.

─── Step 2: Add ANTHROPIC_API_KEY to GitHub Secrets ───

Go to: https://github.com/%s/%s/settings/secrets/actions
Add secret: ANTHROPIC_API_KEY = <your Anthropic API key>

─── Step 3: Enable PR creation ───

Go to: https://github.com/%s/%s/settings/actions
Scroll to "Workflow permissions"
Check: "Allow GitHub Actions to create and approve pull requests"

─── Step 4: Create Jira Automation Rule ───

Go to: Project Settings → Automation → Create rule

Trigger:
  Type: Field value changed
  Field: Assignee
  Condition: Assignee equals %q

Action:
  Type: Send web request
  URL: https://api.github.com/repos/%s/%s/dispatches
  Method: POST
  Headers:
    Authorization: Bearer <paste your GitHub PAT here>
    Content-Type: application/json
  Body:
{
  "event_type": "jerry-ticket",
  "client_payload": {
    "type": "ticket",
    "source": "jira",
    "intent": "{{issue.summary}}",
    "raw_payload": {
      "key": "{{issue.key}}",
      "summary": "{{issue.summary}}",
      "description": "{{issue.description}}"
    }
  }
}

─── Done ───

Assign any Jira ticket to %q and it will:
1. Trigger GitHub Actions
2. Jerry plans and implements the change
3. A pull request is opened for your review
`, owner, repo, workflow, assignee,
		owner, repo,
		owner, repo,
		assignee,
		owner, repo,
		assignee)
}

func printJiraGitLab(owner, repo, host, projectID, workflow, assignee string) {
	if host == "" {
		host = "gitlab.com"
	}
	fmt.Printf(`Jerry Setup — Jira → GitLab CI

Repository: %s/%s
CI Platform: GitLab CI
Workflow:    %s
Trigger:     Assign ticket to %q

─── Step 1: Create a GitLab Pipeline Trigger Token ───

Go to: https://%s/%s/%s/-/settings/ci_cd
Expand "Pipeline trigger tokens"
Create a new token. Save it for the Jira automation rule.

─── Step 2: Add CI/CD Variables ───

Go to: https://%s/%s/%s/-/settings/ci_cd
Expand "Variables"
Add: ANTHROPIC_API_KEY = <your Anthropic API key> (masked)
Add: GITLAB_TOKEN = <a project access token with api scope> (masked)

─── Step 3: Create Jira Automation Rule ───

Go to: Project Settings → Automation → Create rule

Trigger:
  Type: Field value changed
  Field: Assignee
  Condition: Assignee equals %q

Action:
  Type: Send web request
  URL: https://%s/api/v4/projects/%s/trigger/pipeline
  Method: POST
  Headers:
    Content-Type: application/x-www-form-urlencoded
  Body (form-encoded):
    token=<paste your pipeline trigger token>
    ref=main
    variables[JERRY_TICKET_SOURCE]=jira
    variables[JERRY_TICKET_INTENT]={{issue.summary}}
    variables[JERRY_TICKET_KEY]={{issue.key}}
    variables[JERRY_TICKET_DESCRIPTION]={{issue.description}}

─── Done ───

Assign any Jira ticket to %q and it will:
1. Trigger GitLab CI pipeline
2. Jerry plans and implements the change
3. A merge request is opened for your review
`, owner, repo, workflow, assignee,
		host, owner, repo,
		host, owner, repo,
		assignee,
		host, projectID,
		assignee)
}

func detectRepo() (owner, name, host string) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", "", ""
	}
	url := strings.TrimSpace(string(out))
	return parseRepoURL(url)
}

func parseRepoURL(url string) (owner, name, host string) {
	url = strings.TrimSuffix(url, ".git")

	if strings.HasPrefix(url, "git@") {
		url = strings.TrimPrefix(url, "git@")
		hostAndPath := strings.SplitN(url, ":", 2)
		if len(hostAndPath) != 2 {
			return "", "", ""
		}
		host = hostAndPath[0]
		parts := strings.SplitN(hostAndPath[1], "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], host
		}
		return "", "", ""
	}

	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		url = strings.TrimPrefix(url, "https://")
		url = strings.TrimPrefix(url, "http://")
		segments := strings.SplitN(url, "/", 3)
		if len(segments) == 3 {
			return segments[1], segments[2], segments[0]
		}
		return "", "", ""
	}

	return "", "", ""
}

func detectCIPlatform() string {
	if _, err := os.Stat(".github"); err == nil {
		return "github"
	}
	if _, err := os.Stat(".gitlab-ci.yml"); err == nil {
		return "gitlab"
	}
	return ""
}

func listSetupWorkflows(app *App) []string {
	if app.JerryDir == "" {
		return nil
	}
	project, err := spec.LoadProject(app.JerryDir)
	if err != nil {
		return nil
	}
	names := make([]string, len(project.Workflows))
	for i, w := range project.Workflows {
		names[i] = w.Name
	}
	return names
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
