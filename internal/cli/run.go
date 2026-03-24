// motif run: executes a pipeline by name.

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/motif/internal/contextstore"
	"github.com/kilupskalvis/motif/internal/output"
)

func newRunCmd(app *App) *cobra.Command {
	var (
		intent  string
		dryRun  bool
		verbose bool
		quiet   bool
	)

	cmd := &cobra.Command{
		Use:   "run <pipeline>",
		Short: "Execute a pipeline",
		Long:  "Loads and executes a pipeline by name from .motif/pipelines/.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if verbose {
				app.Printer.SetVerbosity(output.VerbosityVerbose)
			} else if quiet {
				app.Printer.SetVerbosity(output.VerbosityQuiet)
			}

			if dryRun {
				return dryRunPipeline(app, args[0], intent)
			}
			return runPipeline(cmd.Context(), app, args[0], intent)
		},
	}

	cmd.Flags().StringVar(&intent, "intent", "", "Trigger intent description")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview pipeline without executing")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed output including tool arguments")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Show only errors and final result")

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

func dryRunPipeline(app *App, pipelineName, intent string) error {
	if app.Loader == nil {
		fmt.Fprintln(os.Stderr, "motif: error: not in a Motif project (no .motif/ directory found)")
		os.Exit(1)
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

	fmt.Fprintln(os.Stderr, "\nValidation: ✓ pipeline is valid")
	fmt.Fprintf(os.Stderr, "\nTo execute: motif run %s", pipelineName)
	if intent != "" {
		fmt.Fprintf(os.Stderr, " --intent %q", intent)
	}
	fmt.Fprintln(os.Stderr)

	return nil
}
