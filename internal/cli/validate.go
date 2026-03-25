package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/agent"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/pipeline"
)

// @lattice:flow validate
func newValidateCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [pipeline]",
		Short: "Validate pipelines and agent definitions",
		Long:  "Validates pipeline YAML and referenced agent definitions. Validates all pipelines if none specified.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if app.Loader == nil || app.AgentLoader == nil {
				return jerrerr.New(jerrerr.CodeJerryDirNotFound,
					"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
			}

			if len(args) == 1 {
				errs := validatePipeline(app.Loader, app.AgentLoader, args[0])
				return reportValidation(app, map[string][]string{args[0]: errs})
			}

			names := app.Loader.ListPipelines()
			if len(names) == 0 {
				return fmt.Errorf("no pipelines found in .jerry/pipelines/")
			}

			results := make(map[string][]string, len(names))
			for _, name := range names {
				results[name] = validatePipeline(app.Loader, app.AgentLoader, name)
			}
			return reportValidation(app, results)
		},
	}
}

// validatePipeline loads a pipeline and all its referenced agents, returning any errors.
func validatePipeline(loader *pipeline.Loader, agentLoader *agent.Loader, name string) []string {
	p, loadErr := loader.Load(name)
	if loadErr != nil {
		return []string{loadErr.Error()}
	}
	return validateAgents(agentLoader, p)
}

// validateAgents checks that all agent steps in a pipeline have valid agent definitions.
func validateAgents(agentLoader *agent.Loader, p *pipeline.Pipeline) []string {
	var errs []string
	for _, step := range p.Steps {
		if step.Agent == "" {
			continue
		}
		if _, agentErr := agentLoader.Load(step.Agent); agentErr != nil {
			errs = append(errs, fmt.Sprintf("step %q: %s", step.Name, agentErr))
		}
	}
	return errs
}

func reportValidation(app *App, results map[string][]string) error {
	hasErrors := false
	for name, errs := range results {
		if len(errs) == 0 {
			app.Printer.ValidationResult(name, true, "valid")
		} else {
			hasErrors = true
			for _, e := range errs {
				app.Printer.ValidationResult(name, false, e)
			}
		}
	}
	if hasErrors {
		return fmt.Errorf("validation failed")
	}
	return nil
}
