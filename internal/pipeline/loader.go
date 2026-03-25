// Pipeline YAML loader with two-pass validation (structural then semantic).

package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kilupskalvis/jerry/internal/errors"
)

// reservedOutputKeys are context keys that steps cannot use as output_key.
var reservedOutputKeys = map[string]struct{}{
	"protocol_version": {},
	"run_id":           {},
	"trigger":          {},
}

var validBackoffStrategies = map[string]struct{}{
	"fixed":       {},
	"exponential": {},
}

// Loader reads and validates pipeline YAML files.
type Loader struct {
	jerryDir string
}

// NewLoader creates a loader that reads pipelines from the given .jerry/ directory.
func NewLoader(jerryDir string) *Loader {
	return &Loader{jerryDir: jerryDir}
}

// ListPipelines returns the names of all pipelines in the pipelines directory.
func (l *Loader) ListPipelines() []string {
	pipelinesDir := filepath.Join(l.jerryDir, "pipelines")
	entries, err := os.ReadDir(pipelinesDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		for _, ext := range []string{".yaml", ".yml"} {
			if strings.HasSuffix(name, ext) {
				names = append(names, strings.TrimSuffix(name, ext))
				break
			}
		}
	}
	return names
}

// Load reads and validates a pipeline by name.
// Looks for .jerry/pipelines/<name>.yaml, then .jerry/pipelines/<name>.yml.
func (l *Loader) Load(name string) (*Pipeline, error) {
	pipelinesDir := filepath.Join(l.jerryDir, "pipelines")

	// Try .yaml first, then .yml
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(pipelinesDir, name+ext)
		if _, statErr := os.Stat(path); statErr == nil {
			return l.LoadFile(path)
		}
	}

	return nil, errors.New(errors.CodePipelineNotFound,
		fmt.Sprintf("pipeline %q not found in %s", name, pipelinesDir))
}

// LoadFile reads and validates a pipeline from a specific file path.
func (l *Loader) LoadFile(path string) (*Pipeline, error) {
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, errors.Wrap(errors.CodeInvalidPipeline,
			fmt.Sprintf("failed to read pipeline file %q", path), readErr)
	}

	var p Pipeline
	if unmarshalErr := yaml.Unmarshal(content, &p); unmarshalErr != nil {
		return nil, errors.Wrap(errors.CodeInvalidPipeline,
			fmt.Sprintf("invalid YAML syntax in %q", path), unmarshalErr)
	}

	if validationErrs := l.validate(&p); len(validationErrs) > 0 {
		return nil, errors.New(errors.CodeInvalidPipeline,
			strings.Join(validationErrs, "; "))
	}

	absPath, _ := filepath.Abs(path)
	p.SourceFile = absPath

	// Pre-resolve agent paths relative to .jerry/ so execution doesn't
	// depend on the working directory.
	l.resolveAgentPaths(&p)

	return &p, nil
}

// resolveAgentPaths rewrites relative agent paths on all steps to absolute
// paths resolved against the .jerry/ directory.
func (l *Loader) resolveAgentPaths(p *Pipeline) {
	for i := range p.Steps {
		if p.Steps[i].Agent != "" {
			p.Steps[i].Agent = l.resolveAgentPath(p.Steps[i].Agent)
		}
		if p.Steps[i].Fallback != nil && p.Steps[i].Fallback.Agent != "" {
			p.Steps[i].Fallback.Agent = l.resolveAgentPath(p.Steps[i].Fallback.Agent)
		}
	}
}

// JerryDir returns the .jerry/ directory path this loader reads from.
func (l *Loader) JerryDir() string {
	return l.jerryDir
}

// validate runs structural and semantic validation on a pipeline.
// Returns a slice of error messages (empty if valid).
func (l *Loader) validate(p *Pipeline) []string {
	var errs []string

	// Structural validation
	if p.Name == "" {
		errs = append(errs, "pipeline must have a 'name' field")
	}
	if p.Steps == nil {
		errs = append(errs, fmt.Sprintf("pipeline %q must have a 'steps' field", p.Name))
		return errs // can't validate steps if nil
	}
	if len(p.Steps) == 0 {
		errs = append(errs, fmt.Sprintf("pipeline %q must have at least one step", p.Name))
		return errs
	}

	// Per-step validation
	stepNames := make(map[string]struct{})
	outputKeys := make(map[string]string) // output_key → step name

	for i := range p.Steps {
		step := &p.Steps[i]

		// Step name
		if step.Name == "" {
			errs = append(errs, fmt.Sprintf("step at index %d is missing a 'name' field", i))
			continue
		}

		// Unique step names
		if _, dup := stepNames[step.Name]; dup {
			errs = append(errs, fmt.Sprintf("duplicate step name: %q", step.Name))
		}
		stepNames[step.Name] = struct{}{}

		// Exactly one executor
		executorCount := countExecutors(step)
		if executorCount == 0 {
			errs = append(errs, fmt.Sprintf("step %q must define exactly one of: agent, script, gate, parallel", step.Name))
		} else if executorCount > 1 {
			errs = append(errs, fmt.Sprintf("step %q must define exactly one of: agent, script, gate, parallel (got multiple)", step.Name))
		}

		// Script validation
		if step.Script != "" && strings.TrimSpace(step.Script) == "" {
			errs = append(errs, fmt.Sprintf("step %q: script must not be empty", step.Name))
		}

		// Agent file existence
		if step.Agent != "" {
			agentPath := l.resolveAgentPath(step.Agent)
			if _, statErr := os.Stat(agentPath); os.IsNotExist(statErr) {
				errs = append(errs, fmt.Sprintf("step %q: agent file %q does not exist", step.Name, step.Agent))
			}
		}

		// Retries
		if step.Retries < 0 {
			errs = append(errs, fmt.Sprintf("step %q: retries must be >= 0", step.Name))
		}

		// Retry backoff
		if _, valid := validBackoffStrategies[step.RetryBackoffStrategy]; step.RetryBackoffStrategy != "" && !valid {
			errs = append(errs, fmt.Sprintf("step %q: retry_backoff must be 'fixed' or 'exponential'", step.Name))
		}

		// Output key validation
		if step.OutputKey != "" {
			if _, reserved := reservedOutputKeys[step.OutputKey]; reserved {
				errs = append(errs, fmt.Sprintf("step %q: output_key %q is reserved (reserved keys: protocol_version, run_id, trigger)", step.Name, step.OutputKey))
			}
			if existingStep, ok := outputKeys[step.OutputKey]; ok {
				errs = append(errs, fmt.Sprintf("steps %q and %q have conflicting output_key: %q", existingStep, step.Name, step.OutputKey))
			}
			outputKeys[step.OutputKey] = step.Name
		}

		// Fallback validation
		if step.Fallback != nil {
			if step.Fallback.Script != "" && strings.TrimSpace(step.Fallback.Script) == "" {
				errs = append(errs, fmt.Sprintf("step %q: fallback script must not be empty", step.Name))
			}
			if step.Fallback.Agent != "" {
				fallbackPath := l.resolveAgentPath(step.Fallback.Agent)
				if _, statErr := os.Stat(fallbackPath); os.IsNotExist(statErr) {
					errs = append(errs, fmt.Sprintf("step %q: fallback agent file %q does not exist", step.Name, step.Fallback.Agent))
				}
			}
		}
	}

	return errs
}

// resolveAgentPath resolves an agent file path relative to the .jerry/ directory.
func (l *Loader) resolveAgentPath(agentRef string) string {
	if filepath.IsAbs(agentRef) {
		return agentRef
	}
	// Agent paths are relative to the .jerry/ directory
	return filepath.Join(l.jerryDir, agentRef)
}

// countExecutors returns how many executor fields are set on a step.
func countExecutors(step *Step) int {
	count := 0
	if step.Script != "" {
		count++
	}
	if step.Agent != "" {
		count++
	}
	if step.Gate {
		count++
	}
	if len(step.Parallel) > 0 {
		count++
	}
	return count
}
