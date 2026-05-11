package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

// CustomToolDef is the YAML structure for a custom tool definition file.
type CustomToolDef struct {
	Description string                     `yaml:"description"`
	Parameters  map[string]CustomToolParam `yaml:"parameters,omitempty"`
	Run         string                     `yaml:"run"`
}

// CustomToolParam is a simplified parameter definition (Agent SDK style).
type CustomToolParam struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

// LoadCustomTools discovers and parses all .yaml files in the given directory.
func LoadCustomTools(toolsDir, repoRoot string, secretEnv []string) ([]Tool, error) {
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read tools directory %q: %w", toolsDir, err)
	}

	var tools []Tool
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		path := filepath.Join(toolsDir, entry.Name())

		t, loadErr := loadCustomTool(name, path, repoRoot, secretEnv)
		if loadErr != nil {
			return nil, fmt.Errorf("tool %q: %w", name, loadErr)
		}
		tools = append(tools, t)
	}
	return tools, nil
}

func loadCustomTool(name, path, repoRoot string, secretEnv []string) (Tool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}

	var def CustomToolDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	if def.Description == "" {
		return nil, fmt.Errorf("missing required field 'description'")
	}
	if def.Run == "" {
		return nil, fmt.Errorf("missing required field 'run'")
	}

	schema := buildParamSchema(def.Parameters)

	return NewToolFunc(name, def.Description, schema, func(ctx context.Context, input json.RawMessage) (string, error) {
		var args map[string]any
		if err := json.Unmarshal(input, &args); err != nil {
			return fmt.Sprintf("Error: invalid input: %v", err), nil
		}

		env := buildCustomToolEnv(args, secretEnv)

		cmd := exec.Command("/bin/sh", "-c", def.Run)
		cmd.Dir = repoRoot
		cmd.Env = env
		cmd.Stdin = bytes.NewReader(input)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		var combined bytes.Buffer
		cmd.Stdout = &combined
		cmd.Stderr = &combined

		if startErr := cmd.Start(); startErr != nil {
			return fmt.Sprintf("Error starting command: %s", startErr), nil
		}

		waitDone := make(chan error, 1)
		go func() { waitDone <- cmd.Wait() }()

		var runErr error
		select {
		case runErr = <-waitDone:
		case <-ctx.Done():
			terminateProcessGroup(cmd)
			<-waitDone
			return "Error: tool execution timed out", nil
		}

		output := combined.String()
		if runErr != nil {
			exitCode := -1
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			}
			if output != "" {
				return fmt.Sprintf("Error (exit code %d): %s", exitCode, output), nil
			}
			return fmt.Sprintf("Error: command failed (exit code %d)", exitCode), nil
		}

		if output == "" {
			return "(no output)", nil
		}
		return output, nil
	}), nil
}

func buildParamSchema(params map[string]CustomToolParam) json.RawMessage {
	if len(params) == 0 {
		return json.RawMessage(`{"type": "object", "properties": {}}`)
	}

	properties := make(map[string]map[string]string)
	var required []string

	for name, p := range params {
		prop := map[string]string{"type": p.Type}
		if prop["type"] == "" {
			prop["type"] = "string"
		}
		if p.Description != "" {
			prop["description"] = p.Description
		}
		properties[name] = prop
		if p.Required {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	data, _ := json.Marshal(schema)
	return data
}

func buildCustomToolEnv(args map[string]any, secretEnv []string) []string {
	env := buildCleanEnv(secretEnv)
	for key, val := range args {
		envKey := "TOOL_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		env = append(env, envKey+"="+fmt.Sprintf("%v", val))
	}
	return env
}
