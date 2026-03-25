// Agent configuration types parsed from markdown frontmatter.

package agent

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/kilupskalvis/jerry/internal/pipeline"
)

const (
	// DefaultTemperature is used when the agent frontmatter does not specify one.
	DefaultTemperature = 0.0

	// DefaultMaxIterations is used when the agent frontmatter does not specify one.
	DefaultMaxIterations = 50
)

// AgentConfig holds the parsed configuration from an agent markdown file.
type AgentConfig struct {
	// From frontmatter
	Name          string            `yaml:"name"`
	Phase         string            `yaml:"phase,omitempty"`
	Model         string            `yaml:"model,omitempty"`
	Temperature   *float64          `yaml:"temperature,omitempty"`
	MaxIterations int               `yaml:"max_iterations,omitempty"`
	Timeout       pipeline.Duration `yaml:"timeout,omitempty"`
	Tools         []ToolAccess      `yaml:"tools,omitempty"`
	ContextAccess []string          `yaml:"context_access"`
	OutputKey     string            `yaml:"output_key"`
	OutputSchema  map[string]any    `yaml:"output_schema"`
	Secrets       []string          `yaml:"secrets,omitempty"`

	// ContextWindow is the model's context window size in tokens.
	// If set, enables proactive compaction at 80% usage.
	// If 0, compaction is purely reactive (triggered by API errors).
	ContextWindow int `yaml:"context_window,omitempty"`

	// Provider overrides prefix-based provider detection for the model.
	// Valid values: "anthropic", "openai". Used for custom or fine-tuned
	// models whose names don't start with a recognized prefix.
	Provider string `yaml:"provider,omitempty"`

	// From markdown body
	Instructions string `yaml:"-"`

	// Resolved at load time
	SourcePath string `yaml:"-"`
}

// ToolAccess represents a tool declaration in the agent frontmatter.
// Can be a simple string (tool name, no constraints) or a map
// (tool name → constraint config).
type ToolAccess struct {
	Name        string
	Constraints map[string]any
}

// UnmarshalYAML handles both string and map forms:
//
//	"read_file"                           → ToolAccess{Name: "read_file"}
//	write_file:
//	  restrict_to: ["src/"]              → ToolAccess{Name: "write_file", Constraints: {...}}
func (t *ToolAccess) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// Simple string form: "read_file"
		t.Name = value.Value
		return nil

	case yaml.MappingNode:
		// Map form: {tool_name: {constraint_key: constraint_value, ...}}
		// A YAML mapping node has alternating key/value children.
		if len(value.Content) != 2 {
			return fmt.Errorf("tool access map must have exactly one key, got %d pairs", len(value.Content)/2)
		}

		t.Name = value.Content[0].Value

		var constraints map[string]any
		if err := value.Content[1].Decode(&constraints); err != nil {
			return fmt.Errorf("cannot parse constraints for tool %q: %w", t.Name, err)
		}
		t.Constraints = constraints
		return nil

	default:
		return fmt.Errorf("tool access must be a string or map, got %v", value.Kind)
	}
}
