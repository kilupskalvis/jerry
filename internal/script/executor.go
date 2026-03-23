// Package script implements the StepExecutor for shell script steps.
// Scripts run in an isolated environment with only declared variables,
// process group management for clean timeout handling, and JSON output
// parsing for context integration.
package script

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/kilupskalvis/motif/internal/errors"
	"github.com/kilupskalvis/motif/internal/pipeline"
)

// GracePeriod is the time to wait after SIGTERM before sending SIGKILL.
const GracePeriod = 5 * time.Second

// Compile-time interface compliance assertion.
var _ pipeline.StepExecutor = (*Executor)(nil)

// Executor runs shell commands for script steps.
type Executor struct {
	repoRoot string
	env      map[string]string
}

// NewExecutor creates a script executor rooted at the given directory
// with the provided base environment variables.
func NewExecutor(repoRoot string, env map[string]string) *Executor {
	return &Executor{
		repoRoot: repoRoot,
		env:      env,
	}
}

// CanExecute returns true if the step has a Script field set.
func (e *Executor) CanExecute(step pipeline.Step) bool {
	return step.Script != ""
}

// Execute runs the script and returns the output.
func (e *Executor) Execute(stepCtx context.Context, step pipeline.Step, store pipeline.ContextReader) (*pipeline.StepOutput, error) {
	startTime := time.Now()

	// Create context file for the script
	contextFilePath, cleanup, contextErr := store.WriteContextFile()
	if contextErr != nil {
		return nil, errors.Wrap(errors.CodeScriptFailed,
			fmt.Sprintf("step %q: failed to write context file", step.Name), contextErr)
	}
	defer cleanup()

	// Build the command — we do NOT use exec.CommandContext because
	// it sends SIGKILL directly. We need process group control to
	// send SIGTERM first, wait for grace period, then SIGKILL.
	cmd := exec.Command("/bin/sh", "-c", step.Script)
	cmd.Dir = e.repoRoot

	// Process group for clean kill — ensures child processes are also terminated.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Build clean environment
	cmd.Env = e.buildEnvironment(step.Name, contextFilePath)

	// Capture stdout and stderr separately
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Start the command
	if startErr := cmd.Start(); startErr != nil {
		return nil, errors.Wrap(errors.CodeScriptFailed,
			fmt.Sprintf("step %q: failed to start script", step.Name), startErr)
	}

	// Wait for command completion or context cancellation
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	var runErr error
	select {
	case runErr = <-waitDone:
		// Command completed (success or failure)
	case <-stepCtx.Done():
		// Context cancelled (timeout or Ctrl+C) — kill process group
		e.killProcessGroup(cmd)
		<-waitDone // Wait for Wait() to return after kill

		duration := time.Since(startTime)
		return nil, errors.New(errors.CodeScriptTimeout,
			fmt.Sprintf("step %q: script timed out after %s", step.Name, duration.Truncate(time.Millisecond)))
	}

	duration := time.Since(startTime)
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// Handle non-zero exit code
	if runErr != nil {
		exitCode := -1
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		return nil, &errors.Error{
			Code: errors.CodeScriptFailed,
			Message: fmt.Sprintf("step %q: script exited with code %d",
				step.Name, exitCode),
			Step:  step.Name,
			Cause: runErr,
		}
	}

	// Parse output
	output := &pipeline.StepOutput{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: 0,
		Duration: duration,
	}

	// If output_key is set, try to parse stdout as JSON
	if step.OutputKey != "" {
		trimmedStdout := strings.TrimSpace(stdout)
		if trimmedStdout != "" {
			var parsed any
			if jsonErr := json.Unmarshal([]byte(trimmedStdout), &parsed); jsonErr == nil {
				output.Data = parsed
			}
			// If JSON parsing fails, Data stays nil — this is not an error
			// (the step succeeded, we just can't merge output into context)
		}
	}

	return output, nil
}

// buildEnvironment constructs a clean environment for the script.
// Only includes PATH, HOME, MOTIF_* variables, and MOTIF_SECRET_* from config.
func (e *Executor) buildEnvironment(stepName string, contextFilePath string) []string {
	envVars := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"MOTIF_RUN_ID=" + "", // Set by engine via store context — extracted below
		"MOTIF_STEP_NAME=" + stepName,
		"MOTIF_CONTEXT_FILE=" + contextFilePath,
	}

	// Add MOTIF_SECRET_* vars from config
	for key, value := range e.env {
		if strings.HasPrefix(key, "MOTIF_SECRET_") {
			envVars = append(envVars, key+"="+value)
		}
	}

	return envVars
}

// killProcessGroup sends SIGTERM to the process group, waits for GracePeriod,
// then sends SIGKILL if the process is still running.
func (e *Executor) killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	pgid, pgidErr := syscall.Getpgid(cmd.Process.Pid)
	if pgidErr != nil {
		return
	}

	// Send SIGTERM to the entire process group
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	// Wait briefly, then force kill
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Process exited gracefully
	case <-time.After(GracePeriod):
		// Force kill
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
