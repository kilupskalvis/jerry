// Workflow YAML loader with validation.

package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kilupskalvis/jerry/internal/errors"
)

// Loader reads and validates workflow definitions from .jerry/ directories.
type Loader struct {
	jerryDir string
}

func NewLoader(jerryDir string) *Loader {
	return &Loader{jerryDir: jerryDir}
}

// Load reads a workflow by name. Resolves to .jerry/<name>/workflow.yaml.
func (l *Loader) Load(name string) (*Workflow, error) {
	workflowFile := filepath.Join(l.jerryDir, name, "workflow.yaml")

	if _, err := os.Stat(workflowFile); os.IsNotExist(err) {
		return nil, errors.New(errors.CodeWorkflowNotFound,
			fmt.Sprintf("workflow %q not found — run 'jerry init' to get started", name))
	}

	return l.LoadFile(workflowFile, name)
}

// LoadFile reads a workflow from a specific file path.
func (l *Loader) LoadFile(path, name string) (*Workflow, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInvalidWorkflow,
			fmt.Sprintf("failed to read workflow file %q", path), err)
	}

	var w Workflow
	if err := yaml.Unmarshal(content, &w); err != nil {
		return nil, errors.Wrap(errors.CodeInvalidWorkflow,
			fmt.Sprintf("invalid YAML in %q", path), err)
	}

	w.Name = name
	absPath, _ := filepath.Abs(path)
	w.SourceFile = absPath

	workflowDir := filepath.Dir(absPath)
	l.deriveStepNames(&w)
	l.resolveAgentPaths(&w, workflowDir)

	if errs := l.validate(&w); len(errs) > 0 {
		return nil, errors.New(errors.CodeInvalidWorkflow,
			strings.Join(errs, "; "))
	}

	return &w, nil
}

// ListWorkflows returns the names of all workflows in .jerry/.
func (l *Loader) ListWorkflows() []string {
	entries, err := os.ReadDir(l.jerryDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		workflowFile := filepath.Join(l.jerryDir, e.Name(), "workflow.yaml")
		if _, err := os.Stat(workflowFile); err == nil {
			names = append(names, e.Name())
		}
	}
	return names
}

func (l *Loader) JerryDir() string {
	return l.jerryDir
}

func (l *Loader) deriveStepNames(w *Workflow) {
	for i := range w.Steps {
		if w.Steps[i].Agent != "" {
			w.Steps[i].Name = w.Steps[i].Agent
		} else {
			w.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
		}
	}
}

func (l *Loader) resolveAgentPaths(w *Workflow, workflowDir string) {
	for i := range w.Steps {
		if w.Steps[i].Agent != "" {
			agentName := w.Steps[i].Agent
			w.Steps[i].Agent = filepath.Join(workflowDir, agentName+".md")
			w.Steps[i].Name = agentName
		}
	}
}

func (l *Loader) validate(w *Workflow) []string {
	var errs []string

	if len(w.Steps) == 0 {
		errs = append(errs, "workflow must have at least one step")
		return errs
	}

	stepNames := make(map[string]struct{})
	for i, step := range w.Steps {
		if step.Agent == "" && step.Run == "" {
			errs = append(errs, fmt.Sprintf("step %d: must define 'agent' or 'run'", i+1))
		}
		if step.Agent != "" && step.Run != "" {
			errs = append(errs, fmt.Sprintf("step %d: cannot define both 'agent' and 'run'", i+1))
		}
		if step.Agent != "" {
			if _, err := os.Stat(step.Agent); os.IsNotExist(err) {
				errs = append(errs, fmt.Sprintf("step %q: agent file %q does not exist",
					step.Name, filepath.Base(step.Agent)))
			}
		}
		if step.Run != "" && strings.TrimSpace(step.Run) == "" {
			errs = append(errs, fmt.Sprintf("step %d: 'run' must not be empty", i+1))
		}
		if step.Retries < 0 {
			errs = append(errs, fmt.Sprintf("step %q: retries must be >= 0", step.Name))
		}
		if _, dup := stepNames[step.Name]; dup {
			errs = append(errs, fmt.Sprintf("duplicate step name: %q", step.Name))
		}
		stepNames[step.Name] = struct{}{}
	}

	return errs
}
