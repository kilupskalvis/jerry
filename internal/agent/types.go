// Agent configuration types parsed from markdown frontmatter.

package agent

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	DefaultTemperature   = 0.0
	DefaultMaxIterations = 50
)

// AgentConfig holds the parsed configuration from an agent markdown file.
type AgentConfig struct {
	Name          string       `yaml:"name"`
	Model         string       `yaml:"model,omitempty"`
	Temperature   *float64     `yaml:"temperature,omitempty"`
	MaxIterations int          `yaml:"max_iterations,omitempty"`
	Tools         []ToolAccess `yaml:"tools,omitempty"`
	Secrets       []string     `yaml:"secrets,omitempty"`
	Provider      string       `yaml:"provider,omitempty"`

	Instructions string `yaml:"-"`
	SourcePath   string `yaml:"-"`
}

// ToolAccess represents a tool declaration in the agent frontmatter.
type ToolAccess struct {
	Name        string
	Constraints map[string]any
}

func (t *ToolAccess) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		t.Name = value.Value
		return nil

	case yaml.MappingNode:
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
