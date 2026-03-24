// motif resume: resumes a failed pipeline from the last checkpoint.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/motif/internal/contextstore"
	motifErrors "github.com/kilupskalvis/motif/internal/errors"
	"github.com/kilupskalvis/motif/internal/pipeline"
	"github.com/kilupskalvis/motif/internal/state"
)

func newResumeCmd(app *App) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "resume <run-id>",
		Short: "Resume a failed pipeline from the last checkpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return resumePipeline(cmd.Context(), app, args[0], force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false,
		"Force resume even if run status is 'running' (for crash recovery)")

	return cmd
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

	// Load the pipeline. Prefer PipelineFile (absolute path) if available,
	// otherwise fall back to loading by name.
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

	// Determine resume point: find the last failed step and restart from there.
	// If the last step result is a failure, re-run it. Otherwise start from the
	// next unexecuted step.
	fromStep := len(runState.StepResults)
	if fromStep > 0 && runState.StepResults[fromStep-1].Status == state.StepFailed {
		fromStep-- // re-run the failed step
	}
	if fromStep >= len(pipelineDef.Steps) {
		return motifErrors.New(motifErrors.CodeRunNotResumable,
			"all steps already completed in saved state")
	}

	// Validate pipeline consistency: step names must match at saved positions.
	for i, saved := range runState.StepResults {
		if i < len(pipelineDef.Steps) && saved.Name != pipelineDef.Steps[i].Name {
			return motifErrors.New(motifErrors.CodePipelineChanged,
				fmt.Sprintf("pipeline structure has changed — step %q at position %d "+
					"does not match saved state (expected %q). Cannot safely resume.",
					pipelineDef.Steps[i].Name, i, saved.Name))
		}
	}

	// Remove the failed step result so RunFrom doesn't produce a duplicate.
	if fromStep < len(runState.StepResults) {
		runState.StepResults = runState.StepResults[:fromStep]
	}

	existingStore := contextstore.RestoreFromSnapshot(runState.Context)
	_, runErr := app.Engine.RunFrom(runCtx, *pipelineDef, fromStep, existingStore, runState)
	return runErr
}
