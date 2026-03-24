// Package pipeline defines the core types and interfaces for Motif pipelines.
// This package is imported by executor implementations (script, agent) and
// by the CLI layer. It does NOT import executor packages — wiring happens
// in cmd/motif/main.go to avoid circular dependencies.
package pipeline

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Pipeline represents a parsed pipeline YAML file.
type Pipeline struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Steps       []Step `yaml:"steps"`

	// SourceFile is the absolute path to the YAML file this pipeline was loaded
	// from. Set by the Loader, used by resume to reload the same pipeline.
	SourceFile string `yaml:"-"`
}

// Step represents a single step in the pipeline.
// Exactly one of Agent, Script, Gate, or Parallel must be set.
// Gate and Parallel are parsed but not executed until Phase 3+.
type Step struct {
	Name         string       `yaml:"name"`
	Agent        string       `yaml:"agent,omitempty"`
	Script       string       `yaml:"script,omitempty"`
	OutputKey    string       `yaml:"output_key,omitempty"`
	Retries      int          `yaml:"retries,omitempty"`
	RetryBackoff string       `yaml:"retry_backoff,omitempty"`
	Timeout      Duration     `yaml:"timeout,omitempty"`
	Fallback     *FallbackDef `yaml:"fallback,omitempty"`
	If           string       `yaml:"if,omitempty"`
	Gate         bool         `yaml:"gate,omitempty"`
	Parallel     []Step       `yaml:"parallel,omitempty"`
}

// FallbackDef defines what to run when a step fails after all retries.
type FallbackDef struct {
	Script string `yaml:"script,omitempty"`
	Agent  string `yaml:"agent,omitempty"`
}

// Duration is a wrapper around time.Duration that supports YAML
// unmarshaling from strings like "30s", "5m", "1h".
type Duration struct {
	time.Duration
}

// UnmarshalYAML implements the yaml.v3 Unmarshaler interface.
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
