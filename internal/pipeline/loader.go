// Pipeline YAML loader with two-pass validation (structural then semantic).

package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kilupskalvis/motif/internal/errors"
)

// reservedOutputKeys are context keys that steps cannot use as output_key.
var reservedOutputKeys = map[string]bool{
	"protocol_version": true,
	"run_id":           true,
	"trigger":          true,
}

// validBackoffStrategies are the allowed values for retry_backoff.
var validBackoffStrategies = map[string]bool{
	"fixed":       true,
	"exponential": true,
}

// Loader reads and validates pipeline YAML files.
type Loader struct {
	motifDir string
}

// NewLoader creates a loader that reads pipelines from the given .motif/ directory.
func NewLoader(motifDir string) *Loader {
	return &Loader{motifDir: motifDir}
}

// Load reads and validates a pipeline by name.
// Looks for .motif/pipelines/<name>.yaml, then .motif/pipelines/<name>.yml.
func (l *Loader) Load(name string) (*Pipeline, error) {
	pipelinesDir := filepath.Join(l.motifDir, "pipelines")

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

	// Pre-resolve agent paths relative to .motif/ so execution doesn't
	// depend on the working directory.
	l.resolveAgentPaths(&p)

	return &p, nil
}

// resolveAgentPaths rewrites relative agent paths on all steps to absolute
// paths resolved against the .motif/ directory.
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

// MotifDir returns the .motif/ directory path this loader reads from.
func (l *Loader) MotifDir() string {
	return l.motifDir
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
	stepNames := make(map[string]bool)
	outputKeys := make(map[string]string) // output_key → step name

	for i := range p.Steps {
		step := &p.Steps[i]

		// Step name
		if step.Name == "" {
			errs = append(errs, fmt.Sprintf("step at index %d is missing a 'name' field", i))
			continue
		}

		// Unique step names
		if stepNames[step.Name] {
			errs = append(errs, fmt.Sprintf("duplicate step name: %q", step.Name))
		}
		stepNames[step.Name] = true

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
		if step.RetryBackoff != "" && !validBackoffStrategies[step.RetryBackoff] {
			errs = append(errs, fmt.Sprintf("step %q: retry_backoff must be 'fixed' or 'exponential'", step.Name))
		}

		// Output key validation
		if step.OutputKey != "" {
			if reservedOutputKeys[step.OutputKey] {
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

// resolveAgentPath resolves an agent file path relative to the .motif/ directory.
func (l *Loader) resolveAgentPath(agentRef string) string {
	if filepath.IsAbs(agentRef) {
		return agentRef
	}
	// Agent paths are relative to the .motif/ directory
	return filepath.Join(l.motifDir, agentRef)
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
