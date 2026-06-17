// Package compile transforms a validated spec.Project into native CI
// configuration. The pipeline: spec → Plan (backend-neutral IR) → backend
// emitter (GitHub Actions YAML, GitLab CI YAML, etc.).
package compile

import (
	"fmt"
	"math"
	"sort"

	"github.com/kilupskalvis/jerry/internal/spec"
)

// Plan is the backend-neutral intermediate representation.
// Backends consume this to emit platform-specific config.
type Plan struct {
	JerryVersion string
	Workflows    []PlannedWorkflow
	Lock         *spec.Lockfile
}

// PlannedWorkflow is one workflow ready for backend emission.
type PlannedWorkflow struct {
	Name     string
	Triggers spec.Triggers
	Env      []string
	Steps    []PlannedStep
}

// PlannedStep is one workflow step (jerry exec invocation).
type PlannedStep struct {
	Label          string
	Command        string
	EnvNames       []string
	TimeoutMinutes int
	Retries        int
}

// GeneratedFile is one file the compiler produces.
type GeneratedFile struct {
	Path    string
	Content []byte
}

// PlanProject transforms a validated project into a backend-neutral Plan IR.
// Workflows are sorted by name for deterministic output.
func PlanProject(project *spec.Project, jerryVersion string) (*Plan, error) {
	wfs := make([]*spec.Workflow, len(project.Workflows))
	copy(wfs, project.Workflows)
	sort.Slice(wfs, func(i, j int) bool { return wfs[i].Name < wfs[j].Name })

	p := &Plan{JerryVersion: jerryVersion, Lock: project.Lock}
	for _, wf := range wfs {
		p.Workflows = append(p.Workflows, planWorkflow(wf))
	}
	return p, nil
}

func planWorkflow(wf *spec.Workflow) PlannedWorkflow {
	pw := PlannedWorkflow{
		Name:     wf.Name,
		Triggers: wf.On,
		Env:      wf.Env,
	}
	for i := range wf.Steps {
		pw.Steps = append(pw.Steps, planStep(wf, &wf.Steps[i]))
	}
	return pw
}

func planStep(wf *spec.Workflow, step *spec.Step) PlannedStep {
	ps := PlannedStep{
		Label:   step.Name,
		Command: fmt.Sprintf("jerry exec %s/%s", wf.Name, step.Name),
		Retries: step.Retries,
	}
	if step.Timeout.Duration > 0 {
		mins := int(math.Ceil(step.Timeout.Seconds() / 60))
		ps.TimeoutMinutes = mins + 1
	}
	ps.EnvNames = resolveEnvNames(wf, step)
	return ps
}

func resolveEnvNames(wf *spec.Workflow, step *spec.Step) []string {
	names := wf.Env
	if step.Env != nil {
		names = *step.Env
	}
	out := make([]string, len(names))
	copy(out, names)
	return out
}
