// Package workflow defines the core types and interfaces for Jerry workflows.
package workflow

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kilupskalvis/jerry/internal/hooks"
)

// Workflow represents a parsed workflow.yaml file.
type Workflow struct {
	Description string      `yaml:"description,omitempty"`
	Steps       []Step      `yaml:"steps"`
	Hooks       hooks.Hooks `yaml:"hooks,omitempty"`

	// Set by the loader, not from YAML.
	Name       string `yaml:"-"`
	SourceFile string `yaml:"-"`
}

// Step represents a single step in the workflow.
// Exactly one of Agent or Run must be set.
type Step struct {
	Agent   string   `yaml:"agent,omitempty"`
	Run     string   `yaml:"run,omitempty"`
	Retries int      `yaml:"retries,omitempty"`
	Timeout Duration `yaml:"timeout,omitempty"`

	Name string `yaml:"name,omitempty"`
}

// Duration is a wrapper around time.Duration that supports YAML
// unmarshaling from strings like "30s", "5m", "1h".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	if raw == "" {
		d.Duration = 0
		return nil
	}
	parsed, parseErr := time.ParseDuration(raw)
	if parseErr != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, parseErr)
	}
	d.Duration = parsed
	return nil
}
