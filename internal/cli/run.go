// jerry run: executes or resumes a workflow.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/agent"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/hooks"
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
		triggerKV    []string
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

			triggerData, resolveErr := resolveTrigger(intent, triggerFile, triggerStdin, triggerKV)
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
	cmd.Flags().StringArrayVar(&triggerKV, "trigger", nil, "Set trigger field (e.g., --trigger type=pull_request)")
	cmd.Flags().StringVar(&resumeRunID, "resume", "", "Resume a failed run by ID")
	cmd.Flags().BoolVar(&force, "force", false, "Force resume even if status is 'running'")

	return cmd
}

func resolveTrigger(intent, triggerFile string, triggerStdin bool, triggerKV []string) (trigger.TriggerData, error) {
	sourcesSet := 0
	if triggerFile != "" {
		sourcesSet++
	}
	if triggerStdin {
		sourcesSet++
	}
	if len(triggerKV) > 0 {
		sourcesSet++
	}
	if sourcesSet > 1 {
		return trigger.TriggerData{}, fmt.Errorf("specify only one trigger source (--trigger-file, --trigger-stdin, or --trigger)")
	}

	switch {
	case triggerFile != "":
		t, err := trigger.FromFile(triggerFile)
		if err != nil {
			return trigger.TriggerData{}, err
		}
		if intent != "" {
			t.Intent = intent
		}
		return *t, nil

	case triggerStdin:
		t, err := trigger.FromReader(os.Stdin)
		if err != nil {
			return trigger.TriggerData{}, err
		}
		if intent != "" {
			t.Intent = intent
		}
		return *t, nil

	case len(triggerKV) > 0:
		t, err := trigger.FromKeyValues(triggerKV)
		if err != nil {
			return trigger.TriggerData{}, err
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

	if len(workflowDef.Hooks) > 0 && app.RepoRoot != "" {
		hookRunner := hooks.NewRunner(workflowDef.Hooks, app.RepoRoot, app.SecretEnv)
		hookRunner.SetBaseEnv(map[string]string{
			"JERRY_HOOK_EVENT":    "",
			"JERRY_HOOK_WORKFLOW": workflowName,
		})
		app.Engine.SetHookRunner(hookRunner)
		if app.AgentExecutor != nil {
			app.AgentExecutor.SetHookRunner(hookRunner)
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
			preview := step.Run
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			fmt.Fprintf(os.Stderr, "  %d. %-16s run     %s\n", i+1, step.Name, preview)
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

// validateAgents pre-flights legacy agent references. Dies with the legacy
// run path in phase 2.
func validateAgents(agentLoader *agent.Loader, w *workflow.Workflow) []string {
	var errs []string
	for _, step := range w.Steps {
		if step.Agent == "" {
			continue
		}
		if _, agentErr := agentLoader.Load(step.Agent); agentErr != nil {
			errs = append(errs, fmt.Sprintf("step %q: %s", step.Name, agentErr))
		}
	}
	return errs
}
