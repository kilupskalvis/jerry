// motif run: executes or resumes a pipeline.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/motif/internal/contextstore"
	motifErrors "github.com/kilupskalvis/motif/internal/errors"
	"github.com/kilupskalvis/motif/internal/pipeline"
	"github.com/kilupskalvis/motif/internal/state"
	"github.com/kilupskalvis/motif/internal/trigger"
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

			// Positional intent: motif run feature "Add health endpoint"
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

	if triggerFile != "" {
		t, err := trigger.FromFile(triggerFile)
		if err != nil {
			return contextstore.TriggerData{}, err
		}
		if intent != "" {
			t.Intent = intent
		}
		return *t, nil
	}

	if triggerStdin {
		t, err := trigger.FromReader(os.Stdin)
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

func runPipeline(runCtx context.Context, app *App, pipelineName string, triggerData contextstore.TriggerData) error {
	if app.Loader == nil || app.Engine == nil {
		return motifErrors.New(motifErrors.CodeMotifDirNotFound,
			"not in a Motif project (no .motif/ directory found) — run 'motif init' to initialize")
	}

	pipelineDef, loadErr := app.Loader.Load(pipelineName)
	if loadErr != nil {
		return loadErr
	}

	if app.AgentLoader != nil {
		for _, step := range pipelineDef.Steps {
			if step.Agent == "" {
				continue
			}
			if _, agentErr := app.AgentLoader.Load(step.Agent); agentErr != nil {
				return fmt.Errorf("pre-flight validation failed: step %q: %w", step.Name, agentErr)
			}
		}
	}

	_, runErr := app.Engine.Run(runCtx, *pipelineDef, triggerData)
	return runErr
}

func dryRunPipeline(app *App, pipelineName, intent string) error {
	if app.Loader == nil {
		return motifErrors.New(motifErrors.CodeMotifDirNotFound,
			"not in a Motif project (no .motif/ directory found) — run 'motif init' to initialize")
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

	fmt.Fprintln(os.Stderr, "\nValidation: pipeline is valid")
	fmt.Fprintf(os.Stderr, "\nTo execute: motif run %s", pipelineName)
	if intent != "" {
		fmt.Fprintf(os.Stderr, " %q", intent)
	}
	fmt.Fprintln(os.Stderr)

	return nil
}

func resumePipeline(runCtx context.Context, app *App, runID string, force bool) error {
	if app.Loader == nil || app.Engine == nil || app.StateStore == nil {
		return motifErrors.New(motifErrors.CodeMotifDirNotFound,
			"not in a Motif project (no .motif/ directory found) — run 'motif init' to initialize")
	}

	runState, loadErr := app.StateStore.LoadRun(runID)
	if loadErr != nil {
		return motifErrors.New(motifErrors.CodeRunNotFound,
			fmt.Sprintf("run %q not found", runID))
	}

	switch runState.Status {
	case state.StatusCompleted:
		return motifErrors.New(motifErrors.CodeRunNotResumable,
			fmt.Sprintf("run %q is already completed — nothing to resume", runID))
	case state.StatusRunning:
		if !force {
			return motifErrors.New(motifErrors.CodeRunNotResumable,
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
		return motifErrors.New(motifErrors.CodeRunNotResumable,
			"all steps already completed in saved state")
	}

	for i, saved := range runState.StepResults {
		if i < len(pipelineDef.Steps) && saved.Name != pipelineDef.Steps[i].Name {
			return motifErrors.New(motifErrors.CodePipelineChanged,
				fmt.Sprintf("pipeline structure has changed — step %q at position %d "+
					"does not match saved state (expected %q). Cannot safely resume.",
					pipelineDef.Steps[i].Name, i, saved.Name))
		}
	}

	if fromStep < len(runState.StepResults) {
		runState.StepResults = runState.StepResults[:fromStep]
	}

	existingStore := contextstore.RestoreFromSnapshot(runState.Context)
	_, runErr := app.Engine.RunFrom(runCtx, *pipelineDef, fromStep, existingStore, runState)
	return runErr
}
