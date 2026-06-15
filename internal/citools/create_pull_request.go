package citools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// CreatePullRequest creates a branch, commits all outstanding changes,
// pushes, and opens a PR. A clean working tree is not an error: it skips
// with a notice and returns successfully.
func (c *Client) CreatePullRequest(repoRoot, title, body, branch string) (string, error) {
	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	if branch == "" {
		branch = "jerry/" + sanitizeBranch(title)
	}

	base, err := gitCmd(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot determine current branch: %w", err)
	}
	if _, err := gitCmd(repoRoot, "checkout", "-b", branch); err != nil {
		return "", fmt.Errorf("cannot create branch %q: %w", branch, err)
	}
	if _, err := gitCmd(repoRoot, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add failed: %w", err)
	}

	status, _ := gitCmd(repoRoot, "status", "--porcelain")
	if strings.TrimSpace(status) == "" {
		_ = gitCmdNoOutput(repoRoot, "checkout", base)
		_ = gitCmdNoOutput(repoRoot, "branch", "-D", branch)
		return "no changes to commit — skipped opening a PR", nil
	}

	if _, err := gitCmd(repoRoot, "commit", "-m", title); err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	pushURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", c.token, c.owner, c.repo)
	if _, err := gitCmd(repoRoot, "push", pushURL, branch); err != nil {
		return "", fmt.Errorf("git push failed: %w", err)
	}

	prURL, number, err := c.openPR(branch, base, title, body)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pull request #%d created: %s", number, prURL), nil
}

// openPR calls the GitHub PR-creation API and parses the response. It is
// separated from the git plumbing so the API contract is unit-testable.
func (c *Client) openPR(branch, base, title, body string) (prURL string, number int, err error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", c.owner, c.repo)
	payload := map[string]string{"title": title, "head": branch, "base": base}
	if body != "" {
		payload["body"] = body
	}
	respBody, err := c.post(path, payload)
	if err != nil {
		return "", 0, err
	}
	var pr struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	if err := json.Unmarshal([]byte(respBody), &pr); err != nil {
		return "", 0, fmt.Errorf("PR created but response was unparseable: %w", err)
	}
	return pr.HTMLURL, pr.Number, nil
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
