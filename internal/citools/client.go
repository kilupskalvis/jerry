// Package citools performs GitHub write actions — PR comments, review
// comments, check statuses, pull requests — for ci: workflow steps. These
// are deterministic, engine-executed actions, not agent tools: the engine
// calls them with already-resolved fields rather than exposing them to a
// model. GitHub REST endpoints here are years-stable, so Jerry owns this
// thin layer to keep the no-dependency, cross-platform runner story.
package citools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

// Config overrides GitHub connection defaults. Zero values fall back to
// the GITHUB_TOKEN environment variable and the public api.github.com.
type Config struct {
	Token   string
	BaseURL string
}

// Client performs GitHub actions against a resolved owner/repo, using the
// triggering event for PR/issue numbers and commit SHAs.
type Client struct {
	owner   string
	repo    string
	token   string
	baseURL string
	trigger *trigger.TriggerData
}

// NewClient resolves the GitHub target (owner/repo from the trigger or the
// GITHUB_REPOSITORY environment variable) and the auth token. It errors
// when no token or repository context is available — ci actions require a
// CI trigger.
func NewClient(t *trigger.TriggerData, cfg Config) (*Client, error) {
	token := cfg.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is required for GitHub actions")
	}
	if t == nil {
		return nil, fmt.Errorf("no trigger context available — ci actions require a CI trigger")
	}

	owner, repo := t.RepoOwner, t.RepoName
	if owner == "" || repo == "" {
		if envRepo := os.Getenv("GITHUB_REPOSITORY"); envRepo != "" {
			if parts := strings.SplitN(envRepo, "/", 2); len(parts) == 2 {
				owner, repo = parts[0], parts[1]
			}
		}
	}
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("cannot determine repository — set GITHUB_REPOSITORY or use a GitHub trigger")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{owner: owner, repo: repo, token: token, baseURL: baseURL, trigger: t}, nil
}

// post sends a JSON POST to baseURL+path and returns the response body,
// erroring on any 4xx/5xx.
func (c *Client) post(path string, body any) (string, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}
