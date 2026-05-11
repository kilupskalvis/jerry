package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const (
	BashTimeout     = 120 * time.Second
	BashGracePeriod = 5 * time.Second
)

func NewBashTool(repoRoot string, secretEnv []string) Tool {
	return NewToolFunc(
		"bash",
		"Run a shell command. Returns combined stdout and stderr. Working directory is the repository root.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "Shell command to execute"
				}
			},
			"required": ["command"]
		}`),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}
			if args.Command == "" {
				return "Error: missing required parameter 'command'", nil
			}

			env := buildCleanEnv(secretEnv)

			timeoutCtx, cancel := context.WithTimeout(ctx, BashTimeout)
			defer cancel()

			cmd := exec.Command("/bin/sh", "-c", args.Command)
			cmd.Dir = repoRoot
			cmd.Env = env
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

			var combined bytes.Buffer
			cmd.Stdout = &combined
			cmd.Stderr = &combined

			if startErr := cmd.Start(); startErr != nil {
				return fmt.Sprintf("Error starting command: %s", startErr), nil
			}

			waitDone := make(chan error, 1)
			go func() { waitDone <- cmd.Wait() }()

			var runErr error
			select {
			case runErr = <-waitDone:
			case <-timeoutCtx.Done():
				terminateProcessGroup(cmd)
				<-waitDone
				return fmt.Sprintf("Command timed out after %s", BashTimeout), nil
			}

			output := combined.String()
			if runErr != nil {
				exitCode := -1
				var exitErr *exec.ExitError
				if errors.As(runErr, &exitErr) {
					exitCode = exitErr.ExitCode()
				}
				if output == "" {
					return fmt.Sprintf("Command failed (exit code %d)", exitCode), nil
				}
				return fmt.Sprintf("Command failed (exit code %d):\n%s", exitCode, output), nil
			}

			if output == "" {
				return "(no output)", nil
			}
			return output, nil
		},
	)
}

func buildCleanEnv(secretEnv []string) []string {
	env := make([]string, 0, 2+len(secretEnv))
	env = append(env, "PATH="+os.Getenv("PATH"), "HOME="+os.Getenv("HOME"))
	env = append(env, secretEnv...)
	return env
}

func terminateProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(BashGracePeriod):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
