package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// AgentRunFunc executes a child agent with the given task and returns its output.
type AgentRunFunc func(ctx context.Context, task string) (string, error)

// AgentTool wraps an agent definition as a Tool that spawns a child agent loop.
type AgentTool struct {
	name         string
	instructions string
	runAgent     AgentRunFunc
}

// NewAgentTool creates a tool that runs a child agent when invoked.
func NewAgentTool(name, instructions string, runAgent AgentRunFunc) *AgentTool {
	return &AgentTool{
		name:         name,
		instructions: instructions,
		runAgent:     runAgent,
	}
}

func (t *AgentTool) Name() string { return t.name }

func (t *AgentTool) Description() string {
	return fmt.Sprintf("Delegate a task to the %s agent. %s", t.name, firstLine(t.instructions))
}

func (t *AgentTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task": {
				"type": "string",
				"description": "Specific task or question for this agent"
			}
		},
		"required": ["task"]
	}`)
}

func (t *AgentTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return fmt.Sprintf("Error: invalid input: %v", err), nil
	}
	if args.Task == "" {
		return "Error: missing required parameter 'task'", nil
	}

	output, err := t.runAgent(ctx, args.Task)
	if err != nil {
		return fmt.Sprintf("Subagent %q failed: %v", t.name, err), nil
	}

	return output, nil
}

func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}
