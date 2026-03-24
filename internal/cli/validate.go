// motif validate: validates pipeline YAML and agent definitions.

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/motif/internal/pipeline"
)

func newValidateCmd(app *App) *cobra.Command {
	var pipelineName string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate pipeline YAML and agent definitions",
		Long:  "Checks pipeline configuration and agent definitions for errors without executing.",
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

	agentErrors := validateAgents(app, pipelineDef.Steps)

	detail := fmt.Sprintf("valid (%d steps)", len(pipelineDef.Steps))
	app.Printer.ValidationResult(name+".yaml", true, detail)

	if len(agentErrors) > 0 {
		for _, errMsg := range agentErrors {
			app.Printer.ValidationResult("  agent", false, errMsg)
		}
		os.Exit(2)
	}

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
			continue
		}

		if result.Pipeline != nil {
			detail := fmt.Sprintf("valid (%d steps)", len(result.Pipeline.Steps))
			app.Printer.ValidationResult(fileName, true, detail)

			agentErrors := validateAgents(app, result.Pipeline.Steps)
			for _, errMsg := range agentErrors {
				app.Printer.ValidationResult("  agent", false, errMsg)
				hasErrors = true
			}
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

// validateAgents loads and validates each agent referenced by pipeline steps.
func validateAgents(app *App, steps []pipeline.Step) []string {
	if app.AgentLoader == nil {
		return nil
	}

	var errs []string
	seen := map[string]bool{}

	for _, step := range steps {
		if step.Agent == "" {
			continue
		}
		if seen[step.Agent] {
			continue
		}
		seen[step.Agent] = true

		_, loadErr := app.AgentLoader.Load(step.Agent)
		if loadErr != nil {
			errs = append(errs, fmt.Sprintf("step %q: %s", step.Name, loadErr))
		}
	}

	return errs
}
