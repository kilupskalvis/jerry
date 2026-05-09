package tool

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

type githubContext struct {
	Owner string
	Repo  string
	Token string
}

func resolveGitHubContext(t *trigger.TriggerData) (*githubContext, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required for GitHub operations")
	}

	if t == nil {
		return nil, fmt.Errorf("no trigger context available — this tool requires a CI trigger (--trigger-file)")
	}

	owner := t.RepoOwner
	repo := t.RepoName
	if owner == "" || repo == "" {
		if envRepo := os.Getenv("GITHUB_REPOSITORY"); envRepo != "" {
			parts := strings.SplitN(envRepo, "/", 2)
			if len(parts) == 2 {
				owner, repo = parts[0], parts[1]
			}
		}
	}
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("cannot determine repository — set GITHUB_REPOSITORY or use a GitHub trigger")
	}

	return &githubContext{Owner: owner, Repo: repo, Token: token}, nil
}

func githubAPI(method, url, token string, body any) (string, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

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
