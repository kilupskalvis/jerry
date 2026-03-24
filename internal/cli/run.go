// motif run: executes a pipeline by name.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/motif/internal/contextstore"
)

func newRunCmd(app *App) *cobra.Command {
	var intent string

	cmd := &cobra.Command{
		Use:   "run <pipeline>",
		Short: "Execute a pipeline",
		Long:  "Loads and executes a pipeline by name from .motif/pipelines/.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipeline(cmd.Context(), app, args[0], intent)
		},
	}

	cmd.Flags().StringVar(&intent, "intent", "", "Trigger intent description")

	return cmd
}

func runPipeline(runCtx context.Context, app *App, pipelineName, intent string) error {
	if app.Loader == nil || app.Engine == nil {
		fmt.Fprintln(os.Stderr, "motif: error: not in a Motif project (no .motif/ directory found)")
		fmt.Fprintln(os.Stderr, "  Run 'motif init' to initialize a new project.")
		os.Exit(1)
	}

	pipelineDef, loadErr := app.Loader.Load(pipelineName)
	if loadErr != nil {
		return loadErr
	}

	trigger := contextstore.TriggerData{
		Type:   "manual",
		Source: "cli",
		Intent: intent,
	}

	_, runErr := app.Engine.Run(runCtx, *pipelineDef, trigger)
	return runErr
}
