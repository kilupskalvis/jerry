// Package compile transforms a validated spec.Project into native CI
// configuration. The pipeline: spec → Plan (backend-neutral IR) → backend
// emitter (GitHub Actions YAML, etc.).
package compile

import (
	"fmt"
	"math"
	"sort"

	"github.com/kilupskalvis/jerry/internal/spec"
)

// Plan is the backend-neutral intermediate representation produced by
// planning. Backends consume this to emit platform-specific config.
// Same spec + same Jerry version = identical Plan (deterministic).
type Plan struct {
	JerryVersion string
	Files        []PlannedFile
}

// PlannedFile is one output file the compiler will write.
type PlannedFile struct {
	Path string
	Jobs []PlannedJob
}

// PlannedJob is one CI job within a file.
type PlannedJob struct {
	Name     string
	Triggers spec.Triggers
	Env      []string
	Steps    []PlannedStep
}

// PlannedStep is one step within a CI job.
type PlannedStep struct {
	Label          string
	Command        string
	EnvRefs        []EnvRef
	TimeoutMinutes int
	Retries        int
	IsPreamble     bool
}

// EnvRef maps a secret name to a platform expression.
type EnvRef struct {
	Name      string
	SecretRef string
}

// PlanProject transforms a validated project into a backend-neutral Plan IR.
// Workflows are sorted by name for deterministic output.
func PlanProject(project *spec.Project, jerryVersion string) (*Plan, error) {
	wfs := make([]*spec.Workflow, len(project.Workflows))
	copy(wfs, project.Workflows)
	sort.Slice(wfs, func(i, j int) bool { return wfs[i].Name < wfs[j].Name })

	p := &Plan{JerryVersion: jerryVersion}
	for _, wf := range wfs {
		f, err := planWorkflow(wf, jerryVersion, project.Lock)
		if err != nil {
			return nil, fmt.Errorf("planning workflow %q: %w", wf.Name, err)
		}
		p.Files = append(p.Files, f)
	}
	return p, nil
}

func planWorkflow(wf *spec.Workflow, jerryVersion string, lock *spec.Lockfile) (PlannedFile, error) {
	job := PlannedJob{
		Name:     wf.Name,
		Triggers: wf.On,
		Env:      wf.Env,
	}

	job.Steps = append(job.Steps, preambleSteps(jerryVersion, lock)...)

	for i := range wf.Steps {
		ps := planStep(wf, &wf.Steps[i])
		job.Steps = append(job.Steps, ps)
	}

	return PlannedFile{
		Path: fmt.Sprintf(".github/workflows/jerry-%s.yml", wf.Name),
		Jobs: []PlannedJob{job},
	}, nil
}

func preambleSteps(jerryVersion string, lock *spec.Lockfile) []PlannedStep {
	steps := []PlannedStep{
		{
			Label:      "Checkout",
			Command:    "actions/checkout@v4",
			IsPreamble: true,
		},
		{
			Label:      "Install Jerry",
			Command:    fmt.Sprintf("curl -sSL https://jerry.dev/install.sh | sh -s -- --version v%s", jerryVersion),
			IsPreamble: true,
		},
	}

	if lock != nil {
		names := make([]string, 0, len(lock.Runtimes))
		for name := range lock.Runtimes {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			rt := lock.Runtimes[name]
			steps = append(steps, PlannedStep{
				Label:      fmt.Sprintf("Install %s", name),
				Command:    fmt.Sprintf("npm install -g %s@%s", rt.Package, rt.Version),
				IsPreamble: true,
			})
		}
	}

	steps = append(steps, PlannedStep{
		Label:      "Drift check",
		Command:    "jerry generate --check",
		IsPreamble: true,
	})
	return steps
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

	ps.EnvRefs = resolveEnvRefs(wf, step)
	return ps
}

func resolveEnvRefs(wf *spec.Workflow, step *spec.Step) []EnvRef {
	names := wf.Env
	if step.Env != nil {
		names = *step.Env
	}

	refs := make([]EnvRef, 0, len(names))
	for _, name := range names {
		refs = append(refs, EnvRef{
			Name:      name,
			SecretRef: fmt.Sprintf("${{ secrets.%s }}", name),
		})
	}
	return refs
}
