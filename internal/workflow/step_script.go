package workflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/run"
)

const ScriptGracePeriod = 5 * time.Second

var _ StepExecutor = (*ScriptExecutor)(nil)

// ScriptExecutor runs shell commands for script steps.
type ScriptExecutor struct {
	repoRoot string
	env      map[string]string
	store    *run.ContextStore
}

// NewScriptExecutor creates a script executor rooted at the given directory.
func NewScriptExecutor(repoRoot string, env map[string]string) *ScriptExecutor {
	return &ScriptExecutor{
		repoRoot: repoRoot,
		env:      env,
	}
}

// SetStore sets the context store for writing context files.
func (e *ScriptExecutor) SetStore(store *run.ContextStore) {
	e.store = store
}

func (e *ScriptExecutor) CanExecute(step Step) bool {
	return step.Run != ""
}

// Execute runs the shell command and returns its stdout as output.
// @lattice:boundary shell
func (e *ScriptExecutor) Execute(ctx context.Context, step Step, prevOutputs []StepOutput) (*StepOutput, error) {
	startTime := time.Now()

	var contextFilePath string
	var cleanup func()
	if e.store != nil {
		var contextErr error
		contextFilePath, cleanup, contextErr = e.store.WriteContextFile()
		if contextErr != nil {
			return nil, jerrerr.Wrap(jerrerr.CodeScriptFailed,
				fmt.Sprintf("step %q: failed to write context file", step.Name), contextErr)
		}
		defer cleanup()
	}

	cmd := exec.Command("/bin/sh", "-c", step.Run)
	cmd.Dir = e.repoRoot
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var runID, intent string
	if e.store != nil {
		snapshot := e.store.Snapshot()
		runID = snapshot.RunID
		intent = snapshot.Trigger.Intent
	}
	cmd.Env = e.buildEnvironment(runID, intent, step.Name, contextFilePath)

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = os.Stderr

	if startErr := cmd.Start(); startErr != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeScriptFailed,
			fmt.Sprintf("step %q: failed to start script", step.Name), startErr)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	var runErr error
	select {
	case runErr = <-waitDone:
	case <-ctx.Done():
		e.killProcessGroup(cmd)
		<-waitDone
		duration := time.Since(startTime)
		return nil, jerrerr.New(jerrerr.CodeScriptTimeout,
			fmt.Sprintf("step %q: script timed out after %s", step.Name, duration.Truncate(time.Millisecond)))
	}

	duration := time.Since(startTime)
	stdout := stdoutBuf.String()

	if runErr != nil {
		exitCode := -1
		exitErr := &exec.ExitError{}
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return nil, &jerrerr.Error{
			Code:    jerrerr.CodeScriptFailed,
			Message: fmt.Sprintf("step %q: script exited with code %d", step.Name, exitCode),
			Step:    step.Name,
			Cause:   runErr,
		}
	}

	return &StepOutput{
		StepName: step.Name,
		Data:     strings.TrimSpace(stdout),
		Duration: duration,
	}, nil
}

func (e *ScriptExecutor) buildEnvironment(runID, intent, stepName, contextFilePath string) []string {
	envVars := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"JERRY_RUN_ID=" + runID,
		"JERRY_INTENT=" + intent,
		"JERRY_STEP_NAME=" + stepName,
		"JERRY_CONTEXT_FILE=" + contextFilePath,
	}

	for key, value := range e.env {
		if strings.HasPrefix(key, "JERRY_SECRET_") {
			envVars = append(envVars, key+"="+value)
		}
	}

	return envVars
}

func (e *ScriptExecutor) killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	pgid, pgidErr := syscall.Getpgid(cmd.Process.Pid)
	if pgidErr != nil {
		return
	}

	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(ScriptGracePeriod):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
