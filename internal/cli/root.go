// Package cli defines the command-line interface for Jerry.
// Commands: init, run, validate, logs. Dependencies are injected via the App struct.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/agent"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/workflow"
)

// App holds the dependencies needed by CLI commands.
// Constructed in main.go and passed to subcommand builders.
type App struct {
	Engine        *workflow.Engine
	Loader        *workflow.Loader
	AgentLoader   *agent.Loader
	AgentExecutor *workflow.AgentExecutor
	StateStore    run.StateStore
	Printer       *output.Printer
	JerryDir      string
	RepoRoot      string
	SecretEnv     []string
}

// NewRootCmd creates the root cobra command with all subcommands.
func NewRootCmd(app *App) *cobra.Command {
	var (
		verbose bool
		quiet   bool
	)

	rootCmd := &cobra.Command{
		Use:   "jerry",
		Short: "Jerry — the agent runtime for CI/CD",
		Long:  "Jerry is the agent runtime for CI/CD. Define AI agents in Markdown, run them as steps in your pipeline.",
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
		newValidateCmd(app),
		newLogsCmd(app),
		newSetupCmd(app),
	)

	return rootCmd
}
