package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newValidateCmd(app *App) *cobra.Command {
	var pipelineName string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate pipeline YAML and agent definitions",
		Long:  "Checks pipeline configuration for errors without executing.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runValidate(app, pipelineName)
		},
	}

	cmd.Flags().StringVar(&pipelineName, "pipeline", "", "Validate a specific pipeline (default: all)")

	return cmd
}

func runValidate(app *App, pipelineName string) error {
	if app.Loader == nil {
		fmt.Fprintln(os.Stderr, "motif: error: not in a Motif project (no .motif/ directory found)")
		fmt.Fprintln(os.Stderr, "  Run 'motif init' to initialize a new project.")
		os.Exit(2)
	}

	if pipelineName != "" {
		return validateSingle(app, pipelineName)
	}
	return validateAll(app)
}

func validateSingle(app *App, name string) error {
	pipelineDef, loadErr := app.Loader.Load(name)
	if loadErr != nil {
		app.Printer.ValidationResult(name+".yaml", false, loadErr.Error())
		os.Exit(2)
	}

	detail := fmt.Sprintf("valid (%d steps)", len(pipelineDef.Steps))
	app.Printer.ValidationResult(name+".yaml", true, detail)
	return nil
}

func validateAll(app *App) error {
	results, loadErr := app.Loader.LoadAll()
	if loadErr != nil {
		return loadErr
	}

	if len(results) == 0 {
		app.Printer.Info("No pipelines found in .motif/pipelines/")
		return nil
	}

	hasErrors := false
	for _, result := range results {
		fileName := filepath.Base(result.Path)

		if len(result.Errors) > 0 {
			for _, errMsg := range result.Errors {
				app.Printer.ValidationResult(fileName, false, errMsg)
			}
			hasErrors = true
		} else if result.Pipeline != nil {
			detail := fmt.Sprintf("valid (%d steps)", len(result.Pipeline.Steps))
			app.Printer.ValidationResult(fileName, true, detail)
		}

		for _, warning := range result.Warnings {
			app.Printer.Warning("%s: %s", fileName, warning)
		}
	}

	if hasErrors {
		os.Exit(2)
	}
	return nil
}
