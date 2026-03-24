// Package tools provides the tool layer for agent execution. It registers
// tools by name, resolves agent tool declarations into executable dispatchers,
// and enforces tool constraints (path restrictions, command allowlists).
//
// Each tool is a function registered with its JSON Schema definition. The
// registry is created once at startup and shared across agent executions.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/kilupskalvis/motif/internal/llm"
)

// Tool defines a single tool that agents can use during the agentic loop.
type Tool struct {
	// Definition is the tool's JSON Schema exposed to the LLM.
	Definition llm.ToolDef

	// Execute runs the tool with the given arguments and returns the result
	// as a string. Tool results are always text in LLM APIs.
	//
	// Expected failures (file not found, constraint violations) are returned
	// as the string result with a nil error. These are fed back to the agent
	// as tool results so it can adapt.
	//
	// Unexpected failures (panics, OS-level errors) return a Go error. The
	// agentic loop wraps these as an error string for the agent.
	Execute func(toolCtx context.Context, args map[string]any) (string, error)
}

// ToolAccess represents a tool declaration from an agent's frontmatter.
// Mirrors agent.ToolAccess to avoid circular imports between agent and tools.
type ToolAccess struct {
	Name        string
	Constraints map[string]any
}

// DispatchFunc executes a tool call by name, enforcing any constraints.
type DispatchFunc func(toolCtx context.Context, call llm.ToolCall) (string, error)

// Registry manages available tools and produces constrained dispatchers
// for individual agent configurations.
type Registry struct {
	tools    map[string]Tool
	repoRoot string
}

// NewRegistry creates a tool registry with all built-in tools registered.
// repoRoot is the working directory for file/command operations.
// env holds environment variables passed to command execution (MOTIF_SECRET_* etc.).
func NewRegistry(repoRoot string, env map[string]string) *Registry {
	r := &Registry{
		tools:    make(map[string]Tool),
		repoRoot: repoRoot,
	}

	// Build env slice for run_command: PATH, HOME, plus MOTIF_* vars.
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
	r.tools[t.Definition.Name] = t
}

// KnownToolNames returns the names of all registered tools.
// Used by the agent loader to validate tool references in agent frontmatter.
func (r *Registry) KnownToolNames() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Resolve returns the tool definitions and a dispatch function for the given
// tool access declarations. Only the requested tools are included in the
// returned defs (the LLM only sees tools the agent declared).
//
// The returned dispatch function enforces any constraints declared in
// toolAccess before executing the tool.
func (r *Registry) Resolve(toolAccess []ToolAccess) ([]llm.ToolDef, DispatchFunc, error) {
	type resolvedTool struct {
		tool        Tool
		constraints map[string]any
	}

	resolved := make(map[string]resolvedTool, len(toolAccess))
	defs := make([]llm.ToolDef, 0, len(toolAccess))

	for _, ta := range toolAccess {
		t, ok := r.tools[ta.Name]
		if !ok {
			known := r.KnownToolNames()
			return nil, nil, fmt.Errorf("unknown tool %q (available: %s)",
				ta.Name, strings.Join(known, ", "))
		}
		resolved[ta.Name] = resolvedTool{
			tool:        t,
			constraints: ta.Constraints,
		}
		defs = append(defs, t.Definition)
	}

	dispatch := func(toolCtx context.Context, call llm.ToolCall) (string, error) {
		rt, ok := resolved[call.Name]
		if !ok {
			return fmt.Sprintf("Error: tool %q is not available to this agent", call.Name), nil
		}

		// Enforce constraints before executing.
		if rt.constraints != nil {
			if violation := ValidateConstraints(call.Name, call.Arguments, rt.constraints, r.repoRoot); violation != "" {
				return violation, nil
			}
		}

		return rt.tool.Execute(toolCtx, call.Arguments)
	}

	return defs, dispatch, nil
}
