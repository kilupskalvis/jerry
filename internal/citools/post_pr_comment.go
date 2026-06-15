package citools

import "fmt"

// PostPRComment posts an issue/PR comment on the triggering pull request.
func (c *Client) PostPRComment(body string) (string, error) {
	if body == "" {
		return "", fmt.Errorf("comment body is required")
	}
	if c.trigger.Number == 0 {
		return "", fmt.Errorf("cannot determine PR/issue number from trigger")
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", c.owner, c.repo, c.trigger.Number)
	if _, err := c.post(path, map[string]string{"body": body}); err != nil {
		return "", err
	}
	return fmt.Sprintf("comment posted on #%d", c.trigger.Number), nil
}
