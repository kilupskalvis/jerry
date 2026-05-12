package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ToolError describes a tool validation problem.
type ToolError struct {
	Tool    string
	Message string
}

func (e ToolError) Error() string {
	return e.Message
}

var baseTools = map[string]bool{
	"bash":       true,
	"read_file":  true,
	"write_file": true,
}

var ciTools = map[string]bool{
	"post_pr_comment":     true,
	"post_review_comment": true,
	"add_check_status":    true,
	"create_pull_request": true,
}

var validParamTypes = map[string]bool{
	"string": true, "integer": true, "number": true, "boolean": true,
}

// CheckTools verifies that each tool name resolves to a built-in or custom tool.
func CheckTools(tools []string, toolsDir string) []ToolError {
	customTools := discoverCustomToolNames(toolsDir)

	var errs []ToolError
	for _, name := range tools {
		if baseTools[name] || ciTools[name] {
			continue
		}
		if customTools[name] {
			continue
		}

		available := make([]string, 0, len(ciTools)+len(customTools))
		for k := range ciTools {
			available = append(available, k)
		}
		for k := range customTools {
			available = append(available, k)
		}

		errs = append(errs, ToolError{
			Tool:    name,
			Message: fmt.Sprintf("tool %q not found (available: %s)", name, strings.Join(available, ", ")),
		})
	}

	return errs
}

// CheckCustomTools validates all .yaml files in the tools directory.
func CheckCustomTools(toolsDir string) []ToolError {
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		return nil
	}

	var errs []ToolError
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(toolsDir, entry.Name())
		errs = append(errs, validateCustomToolFile(entry.Name(), path)...)
	}

	return errs
}

type customToolParamDef struct {
	Type string `yaml:"type"`
}

func validateCustomToolFile(filename, path string) []ToolError {
	data, err := os.ReadFile(path)
	if err != nil {
		return []ToolError{{Tool: filename, Message: fmt.Sprintf("cannot read %s: %v", filename, err)}}
	}

	var def struct {
		Description string                        `yaml:"description"`
		Run         string                        `yaml:"run"`
		Parameters  map[string]customToolParamDef `yaml:"parameters"`
	}
	if err := yaml.Unmarshal(data, &def); err != nil {
		return []ToolError{{Tool: filename, Message: fmt.Sprintf("%s: invalid YAML: %v", filename, err)}}
	}

	var errs []ToolError
	if def.Description == "" {
		errs = append(errs, ToolError{Tool: filename, Message: fmt.Sprintf("%s: missing required field \"description\"", filename)})
	}
	if def.Run == "" {
		errs = append(errs, ToolError{Tool: filename, Message: fmt.Sprintf("%s: missing required field \"run\"", filename)})
	}
	for name, param := range def.Parameters {
		if param.Type != "" && !validParamTypes[param.Type] {
			errs = append(errs, ToolError{
				Tool:    filename,
				Message: fmt.Sprintf("%s: parameter %q has invalid type %q (must be string, integer, number, or boolean)", filename, name, param.Type),
			})
		}
	}

	return errs
}

func discoverCustomToolNames(toolsDir string) map[string]bool {
	names := make(map[string]bool)
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		return names
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			names[strings.TrimSuffix(entry.Name(), ".yaml")] = true
		}
	}
	return names
}
