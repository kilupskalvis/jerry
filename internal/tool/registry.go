package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

// Registry manages available tools and produces constrained dispatchers
// for individual agent configurations.
type Registry struct {
	tools      map[string]Tool
	repoRoot   string
	triggerRef *trigger.TriggerData
}

// NewRegistry creates a tool registry with all built-in tools registered.
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

	r.triggerRef = &trigger.TriggerData{}
	r.register(NewPostPRCommentTool(r.triggerRef))
	r.register(NewPostReviewCommentTool(r.triggerRef))
	r.register(NewAddCheckStatusTool(r.triggerRef))

	return r
}

// SetTrigger injects trigger data for output routing tools.
// Called when the workflow starts and trigger data is available.
func (r *Registry) SetTrigger(t trigger.TriggerData) {
	*r.triggerRef = t
}

func (r *Registry) register(t Tool) {
	r.tools[t.Name()] = t
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
// constraints applied as wrappers.
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

func wrapWithConstraints(inner Tool, constraints map[string]any, repoRoot string) Tool {
	return NewToolFunc(
		inner.Name(),
		inner.Description(),
		inner.Schema(),
		func(ctx context.Context, input json.RawMessage) (string, error) {
			var args map[string]any
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input JSON: %v", err), nil
			}
			if violation := ValidateConstraints(inner.Name(), args, constraints, repoRoot); violation != "" {
				return violation, nil
			}
			return inner.Execute(ctx, input)
		},
	)
}
