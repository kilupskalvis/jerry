package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/kilupskalvis/jerry/internal/llm"
)

const (
	// ToolCommandTimeout is the maximum execution time for a tool command.
	ToolCommandTimeout = 60 * time.Second

	// ToolGracePeriod is the time between SIGTERM and SIGKILL when
	// terminating a timed-out command.
	ToolGracePeriod = 5 * time.Second
)

// NewRunCommandTool creates a run_command tool bound to the given repo root.
// env is a pre-built slice of "KEY=VALUE" strings for the command's environment.
func NewRunCommandTool(repoRoot string, env []string) Tool {
	return Tool{
		Definition: llm.ToolDef{
			Name:        "run_command",
			Description: "Execute a shell command and return its output. Commands run in /bin/sh with a clean environment.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "Shell command to execute",
					},
				},
				"required": []any{"command"},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			command, _ := args["command"].(string)
			if command == "" {
				return "Error: missing required parameter 'command'", nil
			}

			cmdEnv := buildToolEnv(env)

			timeoutCtx, cancel := context.WithTimeout(ctx, ToolCommandTimeout)
			defer cancel()

			cmd := exec.Command("/bin/sh", "-c", command)
			cmd.Dir = repoRoot
			cmd.Env = cmdEnv
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if startErr := cmd.Start(); startErr != nil {
				return fmt.Sprintf("Error starting command: %s", startErr), nil
			}

			waitDone := make(chan error, 1)
			go func() { waitDone <- cmd.Wait() }()

			var runErr error
			select {
			case runErr = <-waitDone:
			case <-timeoutCtx.Done():
				killProcessGroup(cmd)
				<-waitDone
				return fmt.Sprintf("Command timed out after %s", ToolCommandTimeout), nil
			}

			if runErr != nil {
				exitCode := -1
				exitErr := &exec.ExitError{}
				if errors.As(runErr, &exitErr) {
					exitCode = exitErr.ExitCode()
				}
				return fmt.Sprintf("Command failed (exit code %d):\nstdout: %s\nstderr: %s",
					exitCode, stdout.String(), stderr.String()), nil
			}

			out := stdout.String()
			if stderr.Len() > 0 {
				out += stderr.String()
			}
			if out == "" {
				return "(no output)", nil
			}
			return out, nil
		},
	}
}

// buildToolEnv creates a clean environment for tool command execution.
// Only includes PATH, HOME, and any provided extra env vars.
func buildToolEnv(extraEnv []string) []string {
	env := make([]string, 0, 2+len(extraEnv))
	env = append(env, "PATH="+os.Getenv("PATH"), "HOME="+os.Getenv("HOME"))
	env = append(env, extraEnv...)
	return env
}

// killProcessGroup sends SIGTERM to the process group, waits for the grace
// period, then sends SIGKILL if the process is still running.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return
	}

	// Send SIGTERM to the process group.
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	// Wait briefly, then force kill if still alive.
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(ToolGracePeriod):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
