// jerry run: executes a workflow locally — a for-loop over single-step
// exec, NOT an engine. Sequencing in CI belongs to the CI platform.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/exec"
	"github.com/kilupskalvis/jerry/internal/spec"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

// @lattice:flow run
func newRunCmd(app *App) *cobra.Command {
	var (
		keepCtx bool
		ciLive  bool
	)
	cmd := &cobra.Command{
		Use:   "run <workflow> [intent]",
		Short: "Run a workflow locally",
		Long:  "Executes every step of a workflow sequentially on this machine. ci: steps run in preview mode unless --ci-live is set.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			if app.JerryDir == "" {
				return jerrerr.New(jerrerr.CodeJerryDirNotFound,
					"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
			}
			intent := "manual local run"
			if len(args) == 2 {
				intent = args[1]
			}
			err := runLocal(app, args[0], intent, ciLive)
			if keepCtx {
				fmt.Fprintf(os.Stderr, "context dir kept at %s\n", localCtxDir(app))
			} else {
				_ = os.RemoveAll(localCtxDir(app))
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&keepCtx, "keep-ctx", false, "Keep the context directory for inspection")
	cmd.Flags().BoolVar(&ciLive, "ci-live", false, "Execute ci: steps for real instead of preview")
	return cmd
}

func localCtxDir(app *App) string { return filepath.Join(app.RepoRoot, ".jerry-run") }

// runLocal is the local loop: exec each step in order, honoring retries
// (codes 1 and 3 only, same as compiled CI output) and stopping on the
// first terminal failure.
func runLocal(app *App, workflowName, intent string, ciLive bool) error {
	project, err := spec.LoadProject(app.JerryDir)
	if err != nil {
		return err
	}
	var wf *spec.Workflow
	for _, w := range project.Workflows {
		if w.Name == workflowName {
			wf = w
		}
	}
	if wf == nil {
		return jerrerr.New(jerrerr.CodeWorkflowNotFound,
			fmt.Sprintf("workflow %q not found", workflowName))
	}

	ctxDir := localCtxDir(app)
	if err := os.RemoveAll(ctxDir); err != nil {
		return err
	}

	executor := exec.New(exec.Options{
		RepoRoot: app.RepoRoot,
		JerryDir: app.JerryDir,
		Registry: app.Registry,
		Out:      os.Stderr,
	})
	td := trigger.TriggerData{Type: "manual", Source: "cli", Intent: intent}

	for _, step := range wf.Steps {
		attempts := step.Retries + 1
		var code int
		for a := 1; a <= attempts; a++ {
			code = executor.Run(context.Background(), exec.Request{
				Workflow: workflowName,
				Step:     step.Name,
				CtxDir:   ctxDir,
				Trigger:  &td,
				CILive:   ciLive,
			})
			if code == exec.ExitOK || code == exec.ExitConfig || code == exec.ExitBudget {
				break // success or terminal-by-construction
			}
			if a < attempts {
				fmt.Fprintf(os.Stderr, "retrying %s (attempt %d/%d)\n", step.Name, a+1, attempts)
			}
		}
		if code != exec.ExitOK {
			return fmt.Errorf("workflow %q failed at step %q (exit %d)", workflowName, step.Name, code)
		}
	}
	fmt.Fprintf(os.Stderr, "workflow %q completed\n", workflowName)
	return nil
}
