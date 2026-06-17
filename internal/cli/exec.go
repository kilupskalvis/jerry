// jerry exec: run a single workflow step. The per-step entry point that
// compiled CI invokes; exits with the step's exact code (0-4).

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/exec"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

// execTrigger carries the trigger sources from flags/env, resolved lazily.
type execTrigger struct {
	file   string
	pairs  []string
	intent string
}

// @lattice:flow exec
func newExecCmd(app *App) *cobra.Command {
	var (
		t      execTrigger
		ctxDir string
		ciLive bool
	)
	cmd := &cobra.Command{
		Use:   "exec <workflow>/<step>",
		Short: "Run a single workflow step (CI entry point)",
		Long:  "Executes one step of a workflow and exits with its status code. Compiled CI invokes this per step.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir := ctxDir
			if dir == "" {
				dir = filepath.Join(app.RepoRoot, ".jerry-run")
			}
			return runExecCtx(app, args[0], t, ciLive, dir)
		},
	}
	cmd.Flags().StringVar(&t.file, "trigger-file", "", "Path to a CI event payload (defaults to $GITHUB_EVENT_PATH)")
	cmd.Flags().StringArrayVar(&t.pairs, "trigger", nil, "Trigger field key=value (repeatable)")
	cmd.Flags().StringVar(&t.intent, "intent", "", "Manual trigger intent when no event payload is available")
	cmd.Flags().StringVar(&ctxDir, "ctx-dir", "", "Context directory (default <repo>/.jerry-run)")
	cmd.Flags().BoolVar(&ciLive, "ci-live", false, "Execute ci: steps for real instead of preview")
	return cmd
}

// runExec executes one step using the default ctx dir under the repo root.
func runExec(app *App, ref string, t execTrigger, ciLive bool) error {
	return runExecCtx(app, ref, t, ciLive, filepath.Join(app.RepoRoot, ".jerry-run"))
}

func runExecCtx(app *App, ref string, t execTrigger, ciLive bool, ctxDir string) error {
	if app.JerryDir == "" {
		return jerrerr.New(jerrerr.CodeJerryDirNotFound,
			"not in a Jerry project (no .jerry/ directory found)")
	}
	wf, step, ok := strings.Cut(ref, "/")
	if !ok || wf == "" || step == "" {
		return jerrerr.New(jerrerr.CodeInvalidWorkflow,
			fmt.Sprintf("step reference %q must be <workflow>/<step>", ref))
	}

	td, err := resolveExecTrigger(t)
	if err != nil {
		return err
	}

	executor := exec.New(exec.Options{
		RepoRoot: app.RepoRoot,
		JerryDir: app.JerryDir,
		Registry: app.Registry,
		Out:      os.Stderr,
	})
	code := executor.Run(context.Background(), exec.Request{
		Workflow: wf, Step: step, CtxDir: ctxDir, Trigger: td, CILive: ciLive,
	})
	if code != exec.ExitOK {
		return execExit{code: code}
	}
	return nil
}

// resolveExecTrigger picks the trigger source in priority order: explicit
// --trigger-file, then $GITHUB_EVENT_PATH, then --trigger key=value pairs,
// then a manual intent. The Executor only writes it on the first step of a
// run; later steps reuse the recorded trigger.
func resolveExecTrigger(t execTrigger) (*trigger.TriggerData, error) {
	path := t.file
	if path == "" {
		path = os.Getenv("GITHUB_EVENT_PATH")
	}
	if path != "" {
		td, err := trigger.FromFile(path)
		if err != nil {
			return nil, jerrerr.Wrap(jerrerr.CodeConfigInvalid, "reading trigger file", err)
		}
		return td, nil
	}
	if len(t.pairs) > 0 {
		td, err := trigger.FromKeyValues(t.pairs)
		if err != nil {
			return nil, jerrerr.Wrap(jerrerr.CodeConfigInvalid, "parsing --trigger flags", err)
		}
		return td, nil
	}
	return &trigger.TriggerData{Type: "manual", Source: "cli", Intent: t.intent}, nil
}
