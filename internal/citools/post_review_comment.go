package citools

import "fmt"

// PostReviewComment posts an inline review comment on a PR diff line.
func (c *Client) PostReviewComment(file string, line int, body string) (string, error) {
	if file == "" || body == "" || line == 0 {
		return "", fmt.Errorf("path, line, and body are all required")
	}
	if c.trigger.Number == 0 {
		return "", fmt.Errorf("cannot determine PR number from trigger")
	}
	if c.trigger.HeadSHA == "" {
		return "", fmt.Errorf("cannot determine commit SHA from trigger")
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", c.owner, c.repo, c.trigger.Number)
	payload := map[string]any{
		"body":      body,
		"commit_id": c.trigger.HeadSHA,
		"path":      file,
		"line":      line,
	}
	if _, err := c.post(path, payload); err != nil {
		return "", err
	}
	return fmt.Sprintf("review comment posted on %s:%d", file, line), nil
}
