// Package cli defines the command-line interface for Jerry.
// Commands: init, run, logs. Dependencies are injected via the App struct.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/agent"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/pipeline"
	"github.com/kilupskalvis/jerry/internal/state"
)

// App holds the dependencies needed by CLI commands.
// Constructed in main.go and passed to subcommand builders.
type App struct {
	Engine      *pipeline.Engine
	Loader      *pipeline.Loader
	AgentLoader *agent.Loader
	StateStore  state.StateStore
	Printer     *output.Printer
}

// NewRootCmd creates the root cobra command with all subcommands.
func NewRootCmd(app *App) *cobra.Command {
	var (
		verbose bool
		quiet   bool
	)

	rootCmd := &cobra.Command{
		Use:   "jerry",
		Short: "Jerry — composable AI code generation pipelines",
		Long:  "Jerry is an open-source protocol and runtime for composable AI code generation pipelines.",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if verbose {
				app.Printer.SetVerbosity(output.VerbosityVerbose)
			} else if quiet {
				app.Printer.SetVerbosity(output.VerbosityQuiet)
			}
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Show detailed output including tool arguments")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Show only errors and final result")

	rootCmd.AddCommand(
		newInitCmd(),
		newRunCmd(app),
		newLogsCmd(app),
	)

	return rootCmd
}
