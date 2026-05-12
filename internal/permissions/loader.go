package permissions

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type settingsFile struct {
	Permissions rawPermissions `yaml:"permissions"`
}

type rawPermissions struct {
	Deny  []map[string][]string `yaml:"deny,omitempty"`
	Allow []map[string][]string `yaml:"allow,omitempty"`
}

// LoadSettings loads and merges settings.yaml + settings.local.yaml from jerryDir.
func LoadSettings(jerryDir string) (Permissions, error) {
	base, err := loadSettingsFile(filepath.Join(jerryDir, "settings.yaml"))
	if err != nil {
		return Permissions{}, err
	}

	local, localErr := loadSettingsFile(filepath.Join(jerryDir, "settings.local.yaml"))
	if localErr != nil {
		return Permissions{}, localErr
	}

	return base.Merge(local), nil
}

func loadSettingsFile(path string) (Permissions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Permissions{}, nil
		}
		return Permissions{}, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var sf settingsFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return Permissions{}, fmt.Errorf("invalid settings YAML in %s: %w", path, err)
	}

	return rawToPermissions(sf.Permissions), nil
}

func rawToPermissions(raw rawPermissions) Permissions {
	return Permissions{
		Deny:  rawRulesToToolRules(raw.Deny),
		Allow: rawRulesToToolRules(raw.Allow),
	}
}

func rawRulesToToolRules(rawRules []map[string][]string) []ToolRule {
	var rules []ToolRule
	for _, entry := range rawRules {
		for tool, patterns := range entry {
			rules = append(rules, ToolRule{Tool: tool, Patterns: patterns})
		}
	}
	return rules
}

// ParseRawPermissions converts the YAML representation used in agent frontmatter.
func ParseRawPermissions(deny, allow []map[string][]string) Permissions {
	return rawToPermissions(rawPermissions{Deny: deny, Allow: allow})
}
