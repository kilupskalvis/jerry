package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/spec"
)

// @lattice:flow validate
func newValidateCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the .jerry/ spec",
		Long:  "Validates every workflow, settings.yaml, and jerry.lock under .jerry/.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if app.JerryDir == "" {
				return jerrerr.New(jerrerr.CodeJerryDirNotFound,
					"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
			}
			return validateV3(app)
		},
	}
}

// validateV3 validates the whole project, printing issues through the app
// printer. Returns an error when any issue is error-level.
func validateV3(app *App) error {
	project, err := spec.LoadProject(app.JerryDir)
	if err != nil {
		return err
	}
	if len(project.Workflows) == 0 {
		return fmt.Errorf("no workflows found in %s", app.JerryDir)
	}

	issues := spec.ValidateProject(project)
	byWorkflow := map[string][]spec.Issue{}
	for _, is := range issues {
		byWorkflow[is.Workflow] = append(byWorkflow[is.Workflow], is)
	}

	for _, wf := range project.Workflows {
		wfIssues := byWorkflow[wf.Name]
		if len(wfIssues) == 0 {
			app.Printer.ValidationResult(wf.Name, true, "valid")
			continue
		}
		for _, is := range wfIssues {
			prefix := "warning: "
			if is.Level == spec.LevelError {
				prefix = ""
			}
			app.Printer.ValidationResult(wf.Name, is.Level != spec.LevelError, prefix+is.Message)
		}
	}

	if spec.HasErrors(issues) {
		return fmt.Errorf("validation failed")
	}
	return nil
}
