package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ClaudeCodeOptions configures the Claude Code adapter.
type ClaudeCodeOptions struct {
	PinnedVersion string
	Binary        string
}

// ClaudeCode runs the Claude Code CLI as a single-step runtime.
type ClaudeCode struct {
	opts ClaudeCodeOptions
}

// NewClaudeCode constructs the Claude Code adapter.
func NewClaudeCode(opts ClaudeCodeOptions) *ClaudeCode {
	if opts.Binary == "" {
		opts.Binary = "claude"
	}
	return &ClaudeCode{opts: opts}
}

func (c *ClaudeCode) Name() string { return "claude-code" }

func (c *ClaudeCode) Capabilities() Capabilities {
	return Capabilities{
		StructuredOutput: false,
		CostReporting:    true,
		Permissions:      true,
		Streaming:        false,
	}
}

// Invoke spawns claude -p with JSON output, captures stdout, parses the result.
func (c *ClaudeCode) Invoke(ctx context.Context, inv InvocationSpec) (Result, error) {
	if err := c.preflight(); err != nil {
		return Result{}, err
	}

	cmd := exec.CommandContext(ctx, c.opts.Binary, buildClaudeCodeArgs(inv)...)
	cmd.Dir = inv.Workdir
	cmd.Env = claudeCodeEnv(inv.Env)
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("claude-code exited abnormally: %w (stderr: %s)", err, stderr.String())
	}

	return parseClaudeCodeOutput(stdout.Bytes())
}

func (c *ClaudeCode) preflight() error {
	if c.opts.PinnedVersion == "" {
		return nil
	}
	out, err := exec.Command(c.opts.Binary, "--version").Output()
	if err != nil {
		return fmt.Errorf("cannot run %s --version (is claude-code installed?): %w", c.opts.Binary, err)
	}
	got := strings.TrimSpace(string(out))
	if got != c.opts.PinnedVersion {
		return fmt.Errorf("claude-code version mismatch: jerry.lock pins %s but %s is installed — run `jerry lock`",
			c.opts.PinnedVersion, got)
	}
	return nil
}

func claudeCodeEnv(allow []string) []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	return append(env, allow...)
}
