package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/kilupskalvis/jerry/internal/agent"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/permissions"
	"github.com/kilupskalvis/jerry/internal/validation"
	"github.com/kilupskalvis/jerry/internal/workflow"
)

// @lattice:flow validate
func newValidateCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [workflow]",
		Short: "Validate workflows and agent definitions",
		Long:  "Validates workflow YAML and referenced agent definitions. Validates all workflows if none specified.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if app.Loader == nil || app.AgentLoader == nil {
				return jerrerr.New(jerrerr.CodeJerryDirNotFound,
					"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
			}

			results := make(map[string][]string)

			if app.JerryDir != "" {
				if errs := validateSettings(app.JerryDir); len(errs) > 0 {
					results["settings"] = errs
				}

				toolsDir := filepath.Join(app.JerryDir, "tools")
				for _, te := range validation.CheckCustomTools(toolsDir) {
					results["tools/"+te.Tool] = append(results["tools/"+te.Tool], te.Error())
				}
			}

			if len(args) == 1 {
				results[args[0]] = validateWorkflowDeep(app, args[0])
				return reportValidation(app, results)
			}

			names := app.Loader.ListWorkflows()
			if len(names) == 0 && len(results) == 0 {
				return fmt.Errorf("no workflows found in .jerry/")
			}

			for _, name := range names {
				results[name] = validateWorkflowDeep(app, name)
			}
			return reportValidation(app, results)
		},
	}
}

func validateWorkflowDeep(app *App, name string) []string {
	var errs []string

	w, loadErr := app.Loader.Load(name)
	if loadErr != nil {
		return []string{loadErr.Error()}
	}

	toolsDir := filepath.Join(app.JerryDir, "tools")
	workflowDir := filepath.Join(app.JerryDir, name)

	wfPath := filepath.Join(workflowDir, "workflow.yaml")
	errs = append(errs, validateWorkflowSchema(wfPath, toolsDir, workflowDir)...)
	for _, step := range w.Steps {
		if step.Agent == "" {
			continue
		}

		schemaErrs := validateAgentSchema(step.Agent)
		for _, e := range schemaErrs {
			errs = append(errs, fmt.Sprintf("step %q: %s", step.Name, e))
		}

		agentCfg, agentErr := app.AgentLoader.Load(step.Agent)
		if agentErr != nil {
			if len(schemaErrs) == 0 {
				errs = append(errs, fmt.Sprintf("step %q: %s", step.Name, agentErr))
			}
			continue
		}

		toolNames := make([]string, len(agentCfg.Tools))
		for i, ta := range agentCfg.Tools {
			toolNames[i] = ta.Name
		}
		workflowDir := filepath.Join(app.JerryDir, name)
		for _, te := range validation.CheckTools(toolNames, toolsDir, workflowDir) {
			errs = append(errs, fmt.Sprintf("step %q: %s", step.Name, te.Error()))
		}
	}

	return errs
}

func validateWorkflowSchema(path, toolsDir, workflowDir string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}

	knownTools := collectKnownToolNames(toolsDir, workflowDir)

	var errs []string
	for _, fe := range validation.CheckWorkflowFields(raw, knownTools) {
		errs = append(errs, fe.Error())
	}
	return errs
}

func collectKnownToolNames(toolsDir, workflowDir string) []string {
	known := []string{"bash", "read_file", "write_file", "post_pr_comment", "post_review_comment", "add_check_status", "create_pull_request"}
	if entries, err := os.ReadDir(toolsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
				known = append(known, strings.TrimSuffix(e.Name(), ".yaml"))
			}
		}
	}
	if entries, err := os.ReadDir(workflowDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				known = append(known, strings.TrimSuffix(e.Name(), ".md"))
			}
		}
	}
	return known
}

func validateAgentSchema(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	frontmatter, err := extractFrontmatter(string(data))
	if err != nil {
		return nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(frontmatter), &raw); err != nil {
		return nil
	}

	var errs []string
	for _, fe := range validation.CheckAgentFields(raw) {
		errs = append(errs, fe.Error())
	}
	return errs
}

func extractFrontmatter(content string) (string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", fmt.Errorf("no frontmatter")
	}

	rest := content[3:]
	rest = strings.TrimLeft(rest, " \t")
	if rest != "" && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	closeIdx := strings.Index(rest, "\n---")
	if closeIdx == -1 {
		return "", fmt.Errorf("no closing delimiter")
	}

	return rest[:closeIdx], nil
}

func validateAgents(agentLoader *agent.Loader, w *workflow.Workflow) []string {
	var errs []string
	for _, step := range w.Steps {
		if step.Agent == "" {
			continue
		}
		if _, agentErr := agentLoader.Load(step.Agent); agentErr != nil {
			errs = append(errs, fmt.Sprintf("step %q: %s", step.Name, agentErr))
		}
	}
	return errs
}

func validateSettings(jerryDir string) []string {
	_, err := permissions.LoadSettings(jerryDir)
	if err != nil {
		return []string{err.Error()}
	}
	return nil
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
