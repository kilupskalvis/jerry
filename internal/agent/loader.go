// Agent definition loader: parses markdown files with YAML frontmatter.

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	motifErrors "github.com/kilupskalvis/motif/internal/errors"
)

// Loader parses agent markdown definition files into AgentConfig.
type Loader struct {
	knownTools   map[string]bool
	defaultModel string
}

// NewLoader creates an agent loader.
// knownTools is the list of tool names the runtime supports (from Registry.KnownToolNames).
// defaultModel is the fallback model when an agent doesn't specify one (may be empty).
func NewLoader(knownTools []string, defaultModel string) *Loader {
	known := make(map[string]bool, len(knownTools))
	for _, name := range knownTools {
		known[name] = true
	}
	return &Loader{
		knownTools:   known,
		defaultModel: defaultModel,
	}
}

// Load reads and validates an agent markdown file, returning the parsed config.
func (l *Loader) Load(path string) (*AgentConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, motifErrors.Wrap(motifErrors.CodeAgentLoadFailed,
			fmt.Sprintf("cannot resolve path %q", path), err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, motifErrors.Wrap(motifErrors.CodeAgentLoadFailed,
			fmt.Sprintf("cannot read agent file %q", path), err)
	}

	config, parseErr := l.parse(string(data))
	if parseErr != nil {
		return nil, motifErrors.Wrap(motifErrors.CodeAgentLoadFailed,
			fmt.Sprintf("agent %q", path), parseErr)
	}

	config.SourcePath = absPath
	return config, nil
}

// parse splits the markdown file into frontmatter and body, parses the
// frontmatter YAML, applies defaults, and validates.
func (l *Loader) parse(content string) (*AgentConfig, error) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var config AgentConfig
	if yamlErr := yaml.Unmarshal([]byte(frontmatter), &config); yamlErr != nil {
		return nil, fmt.Errorf("invalid frontmatter YAML: %w", yamlErr)
	}

	config.Instructions = strings.TrimSpace(body)

	l.applyDefaults(&config)

	if validErr := l.validate(&config); validErr != nil {
		return nil, validErr
	}

	return &config, nil
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

// applyDefaults fills in missing optional fields with their defaults.
func (l *Loader) applyDefaults(config *AgentConfig) {
	if config.Model == "" {
		config.Model = l.defaultModel
	}

	if config.Temperature == nil {
		temp := DefaultTemperature
		config.Temperature = &temp
	}

	if config.MaxIterations == 0 {
		config.MaxIterations = DefaultMaxIterations
	}
}

// validate checks all required fields and semantic rules.
func (l *Loader) validate(config *AgentConfig) error {
	if config.Name == "" {
		return fmt.Errorf("missing required field 'name'")
	}

	if config.Model == "" {
		return fmt.Errorf("agent %q: no model specified and no global default configured", config.Name)
	}

	if len(config.ContextAccess) == 0 {
		return fmt.Errorf("agent %q: missing required field 'context_access'", config.Name)
	}

	if config.OutputKey == "" {
		return fmt.Errorf("agent %q: missing required field 'output_key'", config.Name)
	}

	if config.OutputSchema == nil {
		return fmt.Errorf("agent %q: missing required field 'output_schema'", config.Name)
	}

	if config.Instructions == "" {
		return fmt.Errorf("agent %q: agent has no instructions (empty markdown body)", config.Name)
	}

	// Validate tool references.
	for _, ta := range config.Tools {
		if !l.knownTools[ta.Name] {
			known := make([]string, 0, len(l.knownTools))
			for name := range l.knownTools {
				known = append(known, name)
			}
			return fmt.Errorf("agent %q: unknown tool %q (available: %s)",
				config.Name, ta.Name, strings.Join(known, ", "))
		}
	}

	// Validate temperature range.
	if config.Temperature != nil && (*config.Temperature < 0.0 || *config.Temperature > 2.0) {
		return fmt.Errorf("agent %q: temperature must be between 0.0 and 2.0", config.Name)
	}

	// Validate max_iterations.
	if config.MaxIterations < 1 {
		return fmt.Errorf("agent %q: max_iterations must be > 0", config.Name)
	}

	// Validate secrets are present in environment.
	for _, secret := range config.Secrets {
		if os.Getenv(secret) == "" {
			return fmt.Errorf("agent %q: required secret %q is not set", config.Name, secret)
		}
	}

	return nil
}
