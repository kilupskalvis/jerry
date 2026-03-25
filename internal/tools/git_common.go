package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitCommandTimeout is the maximum duration for a single git command.
const GitCommandTimeout = 30 * time.Second

// runGit executes a git command in the repo root and returns the trimmed output.
func runGit(ctx context.Context, repoRoot string, args ...string) (string, error) {
	gitCtx, gitCancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer gitCancel()

	cmd := exec.CommandContext(gitCtx, "git", args...)
	cmd.Dir = repoRoot

	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return strings.TrimSpace(string(out)), runErr
	}
	return strings.TrimSpace(string(out)), nil
}

// validateGitPath checks that a file path doesn't escape the repo root.
// Returns an error message string if invalid, empty string if valid.
func validateGitPath(path, repoRoot string) string {
	if path == "" {
		return ""
	}

	absPath, _ := filepath.Abs(filepath.Join(repoRoot, path))
	repoAbs, _ := filepath.Abs(repoRoot)

	if !strings.HasPrefix(absPath, repoAbs+string(filepath.Separator)) && absPath != repoAbs {
		return fmt.Sprintf("Error: path %q escapes the repository root", path)
	}
	return ""
}
