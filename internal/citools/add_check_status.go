package citools

import "fmt"

// AddCheckStatus reports a completed check run on the triggering commit.
// status must be "success" or "failure".
func (c *Client) AddCheckStatus(name, status, summary string) (string, error) {
	if name == "" || status == "" || summary == "" {
		return "", fmt.Errorf("name, status, and summary are all required")
	}
	if status != "success" && status != "failure" {
		return "", fmt.Errorf("status must be 'success' or 'failure', got %q", status)
	}
	if c.trigger.HeadSHA == "" {
		return "", fmt.Errorf("cannot determine commit SHA from trigger")
	}
	path := fmt.Sprintf("/repos/%s/%s/check-runs", c.owner, c.repo)
	payload := map[string]any{
		"name":       name,
		"head_sha":   c.trigger.HeadSHA,
		"status":     "completed",
		"conclusion": status,
		"output":     map[string]string{"title": name, "summary": summary},
	}
	if _, err := c.post(path, payload); err != nil {
		return "", err
	}
	return fmt.Sprintf("check %q reported as %s", name, status), nil
}
