// Package tools provides the tool layer for agent execution.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Tool defines a single tool that agents can use during the agentic loop.
type Tool struct {
	// Name uniquely identifies this tool.
	ToolName string
	// Description explains when the LLM should use this tool.
	ToolDescription string
	// Schema is the JSON Schema for the tool's input parameters.
	Schema json.RawMessage

	// Execute runs the tool with the given input and returns the result
	// as a string. Tool results are always text in LLM APIs.
	//
	// Expected failures (file not found, constraint violations) are returned
	// as the string result with a nil error. These are fed back to the agent
	// as tool results so it can adapt.
	//
	// Unexpected failures (panics, OS-level errors) return a Go error. The
	// agentic loop wraps these as an error string for the agent.
	Execute func(ctx context.Context, input json.RawMessage) (string, error)
}

// ToolAccess represents a tool declaration from an agent's frontmatter.
type ToolAccess struct {
	Name        string
	Constraints map[string]any
}

// Registry manages available tools and produces constrained dispatchers
// for individual agent configurations.
type Registry struct {
	tools    map[string]Tool
	repoRoot string
}

// NewRegistry creates a tool registry with all built-in tools registered.
// repoRoot is the working directory for file/command operations.
// env holds environment variables passed to command execution (JERRY_SECRET_* etc.).
func NewRegistry(repoRoot string, env map[string]string) *Registry {
	r := &Registry{
		tools:    make(map[string]Tool),
		repoRoot: repoRoot,
	}

	envSlice := make([]string, 0, len(env))
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}

	r.register(NewReadFileTool(repoRoot))
	r.register(NewWriteFileTool(repoRoot))
	r.register(NewGlobTool(repoRoot))
	r.register(NewSearchTool(repoRoot))
	r.register(NewRunCommandTool(repoRoot, envSlice))
	r.register(NewListDirectoryTool(repoRoot))
	r.register(NewGitLogTool(repoRoot))
	r.register(NewGitDiffTool(repoRoot))
	r.register(NewGitBlameTool(repoRoot))

	return r
}

func (r *Registry) register(t Tool) {
	r.tools[t.ToolName] = t
}

// KnownToolNames returns the names of all registered tools.
func (r *Registry) KnownToolNames() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Resolve returns the tools for the given tool access declarations, with
// constraints applied as wrappers. Only the requested tools are included.
func (r *Registry) Resolve(toolAccess []ToolAccess) ([]Tool, error) {
	resolved := make([]Tool, 0, len(toolAccess))

	for _, ta := range toolAccess {
		t, ok := r.tools[ta.Name]
		if !ok {
			known := r.KnownToolNames()
			return nil, fmt.Errorf("unknown tool %q (available: %s)",
				ta.Name, strings.Join(known, ", "))
		}

		if ta.Constraints != nil {
			resolved = append(resolved, wrapWithConstraints(t, ta.Constraints, r.repoRoot))
		} else {
			resolved = append(resolved, t)
		}
	}

	return resolved, nil
}

// wrapWithConstraints returns a tool that validates constraints before executing.
func wrapWithConstraints(inner Tool, constraints map[string]any, repoRoot string) Tool {
	return Tool{
		ToolName:        inner.ToolName,
		ToolDescription: inner.ToolDescription,
		Schema:          inner.Schema,
		Execute: func(ctx context.Context, input json.RawMessage) (string, error) {
			var args map[string]any
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input JSON: %v", err), nil
			}
			if violation := ValidateConstraints(inner.ToolName, args, constraints, repoRoot); violation != "" {
				return violation, nil
			}
			return inner.Execute(ctx, input)
		},
	}
}
