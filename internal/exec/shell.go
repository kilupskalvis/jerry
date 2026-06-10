package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kilupskalvis/jerry/internal/handoff"
	"github.com/kilupskalvis/jerry/internal/spec"
)

// runShellStep executes a run: step under /bin/sh with the documented env
// contract: JERRY_* context vars plus PATH/HOME, no ambient secrets.
// Stdout is the step's output; stderr passes through to the operator.
func (e *Executor) runShellStep(ctx context.Context, step *spec.Step,
	dir *handoff.CtxDir, runCtx *handoff.RunContext) (int, error) {

	script, err := handoff.Resolve(step.Run, runCtx)
	if err != nil {
		return ExitConfig, err
	}

	if step.Timeout.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, step.Timeout.Duration)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
	cmd.Dir = e.opts.RepoRoot
	cmd.Env = e.shellEnv(step, dir, runCtx)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = e.opts.Out

	fmt.Fprintf(e.opts.Out, "▸ %s (sh)\n", step.Name)
	runErr := cmd.Run()

	if err := dir.WriteStep(handoff.StepRecord{Name: step.Name, Output: stdout.String()}); err != nil {
		return ExitRuntime, err
	}
	if runErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ExitStep, fmt.Errorf("step %q timed out after %s", step.Name, step.Timeout.Duration)
		}
		return ExitStep, fmt.Errorf("step %q: %w", step.Name, runErr)
	}
	fmt.Fprintf(e.opts.Out, "✓ %s\n", step.Name)
	return ExitOK, nil
}

// shellEnv builds the explicit allowlist env: JERRY_* contract vars,
// PATH/HOME for tool resolution, prior step output files, and declared
// step env names — nothing else.
func (e *Executor) shellEnv(step *spec.Step, dir *handoff.CtxDir, runCtx *handoff.RunContext) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"JERRY_CTX_DIR=" + dir.Root(),
		"JERRY_RUN_ID=" + runCtx.Run.ID,
		"JERRY_STEP_NAME=" + step.Name,
		"JERRY_INTENT=" + runCtx.Trigger.Intent,
	}
	for name := range runCtx.Steps {
		key := "JERRY_STEP_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_OUTPUT_FILE"
		env = append(env, key+"="+dir.StepOutputFile(name))
	}
	if step.Env != nil {
		for _, name := range *step.Env {
			if v, ok := os.LookupEnv(name); ok {
				env = append(env, name+"="+v)
			}
		}
	}
	return env
}
