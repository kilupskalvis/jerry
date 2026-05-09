// Tool interface and ToolFunc adapter for the agent harness.

package agent

import (
	"context"
	"encoding/json"
)

// Tool defines a capability that an agent can invoke during execution.
type Tool interface {
	// Name returns the unique identifier for this tool.
	Name() string
	// Description returns a human-readable explanation used by the LLM
	// to decide when to invoke this tool.
	Description() string
	// Schema returns the JSON Schema describing the tool's input parameters.
	Schema() json.RawMessage
	// Execute runs the tool with the given input and returns a result string
	// for the LLM. Returned errors indicate infrastructure failures and are
	// not sent to the LLM. Business-level errors should be returned in the
	// string with a descriptive message.
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// ToolFunc is a convenience adapter for creating tools from simple functions,
// analogous to http.HandlerFunc.
type ToolFunc struct {
	name        string
	description string
	schema      json.RawMessage
	fn          func(ctx context.Context, input json.RawMessage) (string, error)
}

// NewToolFunc creates a Tool from a name, description, JSON Schema, and execute function.
func NewToolFunc(name, description string, schema json.RawMessage, fn func(ctx context.Context, input json.RawMessage) (string, error)) *ToolFunc {
	return &ToolFunc{
		name:        name,
		description: description,
		schema:      schema,
		fn:          fn,
	}
}

// Name implements Tool.
func (t *ToolFunc) Name() string { return t.name }

// Description implements Tool.
func (t *ToolFunc) Description() string { return t.description }

// Schema implements Tool.
func (t *ToolFunc) Schema() json.RawMessage { return t.schema }

// Execute implements Tool.
func (t *ToolFunc) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	return t.fn(ctx, input)
}
