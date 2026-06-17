// Package cli defines the command-line interface for Jerry.
// Commands: init, run, validate, setup. Dependencies are injected via the
// App struct, constructed in main.go.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/runtime"
)

// App holds the dependencies needed by CLI commands.
type App struct {
	Registry *runtime.Registry
	Printer  *output.Printer
	JerryDir string
	RepoRoot string
}

// NewRootCmd creates the root cobra command with all subcommands.
func NewRootCmd(app *App) *cobra.Command {
	var (
		verbose bool
		quiet   bool
	)

	rootCmd := &cobra.Command{
		Use:   "jerry",
		Short: "Jerry — Terraform for AI agents in CI",
		Long:  "Jerry compiles portable agent-pipeline specs into native CI config and runs them through pluggable agent runtimes.",
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
		newExecCmd(app),
		newValidateCmd(app),
		newLockCmd(app),
		newSetupCmd(app),
	)

	return rootCmd
}
