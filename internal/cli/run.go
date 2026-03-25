// jerry run: executes or resumes a pipeline.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/contextstore"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/pipeline"
	"github.com/kilupskalvis/jerry/internal/state"
	"github.com/kilupskalvis/jerry/internal/trigger"
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
		Use:   "run <pipeline> [intent]",
		Short: "Execute a pipeline",
		Long:  "Run a pipeline by name. Optionally provide an intent as a positional argument.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if resumeRunID != "" {
				return resumePipeline(cmd.Context(), app, resumeRunID, force)
			}

			// Positional intent: jerry run feature "Add health endpoint"
			if len(args) > 1 && intent == "" {
				intent = args[1]
			}

			if dryRun {
				return dryRunPipeline(app, args[0], intent)
			}

			triggerData, resolveErr := resolveTrigger(intent, triggerFile, triggerStdin)
			if resolveErr != nil {
				return resolveErr
			}

			return runPipeline(cmd.Context(), app, args[0], triggerData)
		},
	}

	cmd.Flags().StringVar(&intent, "intent", "", "Trigger intent description")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview pipeline without executing")
	cmd.Flags().StringVar(&triggerFile, "trigger-file", "", "Path to trigger JSON file")
	cmd.Flags().BoolVar(&triggerStdin, "trigger-stdin", false, "Read trigger JSON from stdin")
	cmd.Flags().StringVar(&resumeRunID, "resume", "", "Resume a failed run by ID")
	cmd.Flags().BoolVar(&force, "force", false, "Force resume even if status is 'running'")

	// Hide flags intended for internal/CI use.
	_ = cmd.Flags().MarkHidden("intent")
	_ = cmd.Flags().MarkHidden("trigger-file")
	_ = cmd.Flags().MarkHidden("trigger-stdin")

	return cmd
}

func resolveTrigger(intent, triggerFile string, triggerStdin bool) (contextstore.TriggerData, error) {
	sourcesSet := 0
	if triggerFile != "" {
		sourcesSet++
	}
	if triggerStdin {
		sourcesSet++
	}
	if sourcesSet > 1 {
		return contextstore.TriggerData{}, fmt.Errorf("specify only one trigger source (got multiple of --trigger-file, --trigger-stdin)")
	}

	var t *contextstore.TriggerData
	var err error
	switch {
	case triggerFile != "":
		t, err = trigger.FromFile(triggerFile)
	case triggerStdin:
		t, err = trigger.FromReader(os.Stdin)
	}
	if t != nil {
		if err != nil {
			return contextstore.TriggerData{}, err
		}
		if intent != "" {
			t.Intent = intent
		}
		return *t, nil
	}

	return contextstore.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: intent,
	}, nil
}

// @lattice:flow run
func runPipeline(ctx context.Context, app *App, pipelineName string, triggerData contextstore.TriggerData) error {
	if app.Loader == nil || app.Engine == nil {
		return jerrerr.New(jerrerr.CodeJerryDirNotFound,
			"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
	}

	pipelineDef, loadErr := app.Loader.Load(pipelineName)
	if loadErr != nil {
		return loadErr
	}

	if app.AgentLoader != nil {
		if errs := validateAgents(app.AgentLoader, pipelineDef); len(errs) > 0 {
			return fmt.Errorf("pre-flight validation failed: %s", errs[0])
		}
	}

	_, runErr := app.Engine.Run(ctx, *pipelineDef, triggerData)
	return runErr
}

func dryRunPipeline(app *App, pipelineName, intent string) error {
	if app.Loader == nil {
		return jerrerr.New(jerrerr.CodeJerryDirNotFound,
			"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
	}

	pipelineDef, loadErr := app.Loader.Load(pipelineName)
	if loadErr != nil {
		return loadErr
	}

	fmt.Fprintf(os.Stderr, "Dry run: %s\n", pipelineDef.Name)
	if intent != "" {
		fmt.Fprintf(os.Stderr, "Trigger: manual (intent: %q)\n", intent)
	}

	fmt.Fprintln(os.Stderr, "\nSteps:")
	for i, step := range pipelineDef.Steps {
		if step.Agent != "" {
			fmt.Fprintf(os.Stderr, "  %d. %-16s agent   %s\n", i+1, step.Name, step.Agent)
		} else if step.Script != "" {
			script := step.Script
			if len(script) > 60 {
				script = script[:60] + "..."
			}
			fmt.Fprintf(os.Stderr, "  %d. %-16s script  %s\n", i+1, step.Name, script)
		}
	}

	if app.AgentLoader != nil {
		if errs := validateAgents(app.AgentLoader, pipelineDef); len(errs) > 0 {
			fmt.Fprintln(os.Stderr, "\nValidation errors:")
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  ✗ %s\n", e)
			}
			return fmt.Errorf("dry run failed: validation errors found")
		}
	}
	fmt.Fprintln(os.Stderr, "\nValidation: pipeline and agents are valid")
	fmt.Fprintf(os.Stderr, "\nTo execute: jerry run %s", pipelineName)
	if intent != "" {
		fmt.Fprintf(os.Stderr, " %q", intent)
	}
	fmt.Fprintln(os.Stderr)

	return nil
}

func resumePipeline(ctx context.Context, app *App, runID string, force bool) error {
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
	case state.StatusCompleted:
		return jerrerr.New(jerrerr.CodeRunNotResumable,
			fmt.Sprintf("run %q is already completed — nothing to resume", runID))
	case state.StatusRunning:
		if !force {
			return jerrerr.New(jerrerr.CodeRunNotResumable,
				fmt.Sprintf("run %q has status 'running' — if the process crashed, use --force to resume", runID))
		}
	case state.StatusFailed:
		// Normal case — proceed.
	}

	var pipelineErr error
	var pipelineDef *pipeline.Pipeline
	if runState.PipelineFile != "" {
		pipelineDef, pipelineErr = app.Loader.LoadFile(runState.PipelineFile)
	} else {
		pipelineDef, pipelineErr = app.Loader.Load(runState.PipelineName)
	}
	if pipelineErr != nil {
		return pipelineErr
	}

	fromStep := len(runState.StepResults)
	if fromStep > 0 && runState.StepResults[fromStep-1].Status == state.StepFailed {
		fromStep--
	}
	if fromStep >= len(pipelineDef.Steps) {
		return jerrerr.New(jerrerr.CodeRunNotResumable,
			"all steps already completed in saved state")
	}

	for i, saved := range runState.StepResults {
		if i < len(pipelineDef.Steps) && saved.Name != pipelineDef.Steps[i].Name {
			return jerrerr.New(jerrerr.CodePipelineChanged,
				fmt.Sprintf("pipeline structure has changed — step %q at position %d "+
					"does not match saved state (expected %q). Cannot safely resume.",
					pipelineDef.Steps[i].Name, i, saved.Name))
		}
	}

	if fromStep < len(runState.StepResults) {
		runState.StepResults = runState.StepResults[:fromStep]
	}

	existingStore := contextstore.RestoreFromSnapshot(runState.Context)
	_, runErr := app.Engine.RunFrom(ctx, *pipelineDef, fromStep, existingStore, runState)
	return runErr
}
