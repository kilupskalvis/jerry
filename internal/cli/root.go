// Package cli defines the command-line interface for Motif.
// Each subcommand (init, run, validate) is defined in its own file.
// Dependencies are injected via the App struct.
package cli

import (
	"github.com/kilupskalvis/motif/internal/output"
	"github.com/kilupskalvis/motif/internal/pipeline"
	"github.com/kilupskalvis/motif/internal/state"
	"github.com/spf13/cobra"
)

// App holds the dependencies needed by CLI commands.
// Constructed in main.go and passed to subcommand builders.
type App struct {
	Engine     *pipeline.Engine
	Loader     *pipeline.Loader
	StateStore state.StateStore
	Printer    *output.Printer
}

// NewRootCmd creates the root cobra command with all subcommands.
func NewRootCmd(app *App) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "motif",
		Short: "Motif — composable AI code generation pipelines",
		Long:  "Motif is an open-source protocol and runtime for composable AI code generation pipelines.",
		// No run function — the root command just shows help
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(
		newInitCmd(),
		newRunCmd(app),
		newValidateCmd(app),
	)

	return rootCmd
}
