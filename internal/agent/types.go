// Agent configuration types parsed from markdown frontmatter.

package agent

import (
	"github.com/kilupskalvis/jerry/internal/permissions"
	"github.com/kilupskalvis/jerry/internal/tool"
)

const (
	DefaultTemperature   = 0.0
	DefaultMaxIterations = 50
)

// AgentConfig holds the parsed configuration from an agent markdown file.
type AgentConfig struct {
	Name          string            `yaml:"name"`
	Model         string            `yaml:"model,omitempty"`
	Temperature   *float64          `yaml:"temperature,omitempty"`
	MaxIterations int               `yaml:"max_iterations,omitempty"`
	Tools         []tool.ToolAccess `yaml:"tools,omitempty"`
	Secrets       []string          `yaml:"secrets,omitempty"`
	Provider      string            `yaml:"provider,omitempty"`

	Permissions  permissions.Permissions `yaml:"-"`
	Instructions string                  `yaml:"-"`
	SourcePath   string                  `yaml:"-"`
}
