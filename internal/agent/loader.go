// Agent definition loader: parses markdown files with YAML frontmatter.

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

// Loader parses agent markdown definition files into AgentConfig.
type Loader struct {
	defaultModel string
}

// NewLoader creates an agent loader.
// defaultModel is the fallback model when an agent doesn't specify one (may be empty).
func NewLoader(defaultModel string) *Loader {
	return &Loader{defaultModel: defaultModel}
}

// Load reads and validates an agent markdown file, returning the parsed config.
func (l *Loader) Load(path string) (*AgentConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeAgentLoadFailed,
			fmt.Sprintf("cannot resolve path %q", path), err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeAgentLoadFailed,
			fmt.Sprintf("cannot read agent file %q", path), err)
	}

	agentCfg, parseErr := l.parse(string(data))
	if parseErr != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeAgentLoadFailed,
			fmt.Sprintf("agent %q", path), parseErr)
	}

	agentCfg.SourcePath = absPath
	return agentCfg, nil
}

// parse splits the markdown file into frontmatter and body, parses the
// frontmatter YAML, applies defaults, and validates.
func (l *Loader) parse(content string) (*AgentConfig, error) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var agentCfg AgentConfig
	if yamlErr := yaml.Unmarshal([]byte(frontmatter), &agentCfg); yamlErr != nil {
		return nil, fmt.Errorf("invalid frontmatter YAML: %w", yamlErr)
	}

	agentCfg.Instructions = strings.TrimSpace(body)

	l.applyDefaults(&agentCfg)

	if validErr := l.validate(&agentCfg); validErr != nil {
		return nil, validErr
	}

	return &agentCfg, nil
}

// splitFrontmatter separates YAML frontmatter from the markdown body.
// The file must start with "---" on the first line.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		return "", "", fmt.Errorf("file does not start with frontmatter delimiter '---'")
	}

	// Find the closing delimiter.
	rest := content[3:] // skip opening "---"
	rest = strings.TrimLeft(rest, " \t")
	if rest != "" && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	closeIdx := strings.Index(rest, "\n---")
	if closeIdx == -1 {
		return "", "", fmt.Errorf("missing closing frontmatter delimiter '---'")
	}

	frontmatter = rest[:closeIdx]
	body = rest[closeIdx+4:] // skip "\n---"

	if strings.TrimSpace(frontmatter) == "" {
		return "", "", fmt.Errorf("empty frontmatter")
	}

	return frontmatter, body, nil
}

// applyDefaults fills in missing optional fields.
// Resolution: agent frontmatter → defaultModel env var → hardcoded fallback.
func (l *Loader) applyDefaults(agentCfg *AgentConfig) {
	if agentCfg.Model == "" {
		agentCfg.Model = l.defaultModel
	}

	if agentCfg.MaxIterations == 0 {
		agentCfg.MaxIterations = DefaultMaxIterations
	}

	if agentCfg.Temperature == nil {
		temp := DefaultTemperature
		agentCfg.Temperature = &temp
	}
}

// validate checks all required fields and semantic rules.
func (l *Loader) validate(agentCfg *AgentConfig) error {
	if agentCfg.Name == "" {
		return fmt.Errorf("missing required field 'name'")
	}

	if agentCfg.Model == "" {
		return fmt.Errorf("agent %q: no model specified and no default configured (set JERRY_DEFAULT_MODEL)", agentCfg.Name)
	}

	if agentCfg.Instructions == "" {
		return fmt.Errorf("agent %q: agent has no instructions (empty markdown body)", agentCfg.Name)
	}

	// Validate temperature range.
	if agentCfg.Temperature != nil && (*agentCfg.Temperature < 0.0 || *agentCfg.Temperature > 2.0) {
		return fmt.Errorf("agent %q: temperature must be between 0.0 and 2.0", agentCfg.Name)
	}

	// Validate max_iterations.
	if agentCfg.MaxIterations < 1 {
		return fmt.Errorf("agent %q: max_iterations must be > 0", agentCfg.Name)
	}

	// Validate secrets are present in environment.
	for _, secret := range agentCfg.Secrets {
		if os.Getenv(secret) == "" {
			return fmt.Errorf("agent %q: required secret %q is not set", agentCfg.Name, secret)
		}
	}

	return nil
}
