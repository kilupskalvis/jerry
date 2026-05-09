// jerry run: executes or resumes a workflow.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/trigger"
	"github.com/kilupskalvis/jerry/internal/workflow"
)

func newRunCmd(app *App) *cobra.Command {
	var (
		intent       string
		dryRun       bool
		triggerFile  string
		triggerStdin bool
		resumeRunID  string
		force        bool
	)

	cmd := &cobra.Command{
		Use:   "run <workflow> [intent]",
		Short: "Execute a workflow",
		Long:  "Run a workflow by name. Optionally provide an intent as a positional argument.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if resumeRunID != "" {
				return resumeWorkflow(cmd.Context(), app, resumeRunID, force)
			}

			if len(args) > 1 && intent == "" {
				intent = args[1]
			}

			if dryRun {
				return dryRunWorkflow(app, args[0], intent)
			}

			triggerData, resolveErr := resolveTrigger(intent, triggerFile, triggerStdin)
			if resolveErr != nil {
				return resolveErr
			}

			return runWorkflow(cmd.Context(), app, args[0], triggerData)
		},
	}

	cmd.Flags().StringVar(&intent, "intent", "", "Trigger intent description")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview workflow without executing")
	cmd.Flags().StringVar(&triggerFile, "trigger-file", "", "Path to trigger JSON file")
	cmd.Flags().BoolVar(&triggerStdin, "trigger-stdin", false, "Read trigger JSON from stdin")
	cmd.Flags().StringVar(&resumeRunID, "resume", "", "Resume a failed run by ID")
	cmd.Flags().BoolVar(&force, "force", false, "Force resume even if status is 'running'")

	return cmd
}

func resolveTrigger(intent, triggerFile string, triggerStdin bool) (trigger.TriggerData, error) {
	sourcesSet := 0
	if triggerFile != "" {
		sourcesSet++
	}
	if triggerStdin {
		sourcesSet++
	}
	if sourcesSet > 1 {
		return trigger.TriggerData{}, fmt.Errorf("specify only one trigger source (got multiple of --trigger-file, --trigger-stdin)")
	}

	var t *trigger.TriggerData
	var err error
	switch {
	case triggerFile != "":
		t, err = trigger.FromFile(triggerFile)
	case triggerStdin:
		t, err = trigger.FromReader(os.Stdin)
	}
	if t != nil {
		if err != nil {
			return trigger.TriggerData{}, err
		}
		if intent != "" {
			t.Intent = intent
		}
		return *t, nil
	}

	return trigger.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: intent,
	}, nil
}

// @lattice:flow run
func runWorkflow(ctx context.Context, app *App, workflowName string, triggerData trigger.TriggerData) error {
	if app.Loader == nil || app.Engine == nil {
		return jerrerr.New(jerrerr.CodeJerryDirNotFound,
			"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
	}

	workflowDef, loadErr := app.Loader.Load(workflowName)
	if loadErr != nil {
		return loadErr
	}

	if app.AgentLoader != nil {
		if errs := validateAgents(app.AgentLoader, workflowDef); len(errs) > 0 {
			return fmt.Errorf("pre-flight validation failed: %s", errs[0])
		}
	}

	_, runErr := app.Engine.Run(ctx, *workflowDef, triggerData)
	return runErr
}

func dryRunWorkflow(app *App, workflowName, intent string) error {
	if app.Loader == nil {
		return jerrerr.New(jerrerr.CodeJerryDirNotFound,
			"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
	}

	workflowDef, loadErr := app.Loader.Load(workflowName)
	if loadErr != nil {
		return loadErr
	}

	fmt.Fprintf(os.Stderr, "Dry run: %s\n", workflowDef.Name)
	if intent != "" {
		fmt.Fprintf(os.Stderr, "Trigger: manual (intent: %q)\n", intent)
	}

	fmt.Fprintln(os.Stderr, "\nSteps:")
	for i, step := range workflowDef.Steps {
		if step.Agent != "" {
			fmt.Fprintf(os.Stderr, "  %d. %-16s agent   %s\n", i+1, step.Name, step.Agent)
		} else if step.Run != "" {
			run := step.Run
			if len(run) > 60 {
				run = run[:60] + "..."
			}
			fmt.Fprintf(os.Stderr, "  %d. %-16s run     %s\n", i+1, step.Name, run)
		}
	}

	if app.AgentLoader != nil {
		if errs := validateAgents(app.AgentLoader, workflowDef); len(errs) > 0 {
			fmt.Fprintln(os.Stderr, "\nValidation errors:")
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  ✗ %s\n", e)
			}
			return fmt.Errorf("dry run failed: validation errors found")
		}
	}
	fmt.Fprintln(os.Stderr, "\nValidation: workflow and agents are valid")
	fmt.Fprintf(os.Stderr, "\nTo execute: jerry run %s", workflowName)
	if intent != "" {
		fmt.Fprintf(os.Stderr, " %q", intent)
	}
	fmt.Fprintln(os.Stderr)

	return nil
}

func resumeWorkflow(ctx context.Context, app *App, runID string, force bool) error {
	if app.Loader == nil || app.Engine == nil || app.StateStore == nil {
		return jerrerr.New(jerrerr.CodeJerryDirNotFound,
			"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
	}

	runState, loadErr := app.StateStore.LoadRun(runID)
	if loadErr != nil {
		return jerrerr.New(jerrerr.CodeRunNotFound,
			fmt.Sprintf("run %q not found", runID))
	}

	switch runState.Status {
	case run.StatusCompleted:
		return jerrerr.New(jerrerr.CodeRunNotResumable,
			fmt.Sprintf("run %q is already completed — nothing to resume", runID))
	case run.StatusRunning:
		if !force {
			return jerrerr.New(jerrerr.CodeRunNotResumable,
				fmt.Sprintf("run %q has status 'running' — if the process crashed, use --force to resume", runID))
		}
	case run.StatusFailed:
	}

	name := runState.WorkflowName

	var workflowDef *workflow.Workflow
	var workflowErr error

	if runState.WorkflowFile != "" {
		workflowDef, workflowErr = app.Loader.LoadFile(runState.WorkflowFile, name)
	} else {
		workflowDef, workflowErr = app.Loader.Load(name)
	}
	if workflowErr != nil {
		return workflowErr
	}

	fromStep := len(runState.StepResults)
	if fromStep > 0 && runState.StepResults[fromStep-1].Status == run.StepFailed {
		fromStep--
	}
	if fromStep >= len(workflowDef.Steps) {
		return jerrerr.New(jerrerr.CodeRunNotResumable,
			"all steps already completed in saved state")
	}

	for i, saved := range runState.StepResults {
		if i < len(workflowDef.Steps) && saved.Name != workflowDef.Steps[i].Name {
			return jerrerr.New(jerrerr.CodeWorkflowChanged,
				fmt.Sprintf("workflow structure has changed — step %q at position %d "+
					"does not match saved state (expected %q). Cannot safely resume.",
					workflowDef.Steps[i].Name, i, saved.Name))
		}
	}

	if fromStep < len(runState.StepResults) {
		runState.StepResults = runState.StepResults[:fromStep]
	}

	existingStore := run.RestoreFromSnapshot(runState.Context)
	_, runErr := app.Engine.RunFrom(ctx, *workflowDef, fromStep, existingStore, runState)
	return runErr
}
