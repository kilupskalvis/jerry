// Package tool provides the tool interface, registry, and built-in tool implementations.
package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Tool defines a capability that an agent can invoke during execution.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// ToolFunc is a convenience adapter for creating tools from simple functions.
type ToolFunc struct {
	name        string
	description string
	schema      json.RawMessage
	fn          func(ctx context.Context, input json.RawMessage) (string, error)
}

// NewToolFunc creates a Tool from a name, description, JSON Schema, and execute function.
func NewToolFunc(name, description string, schema json.RawMessage, fn func(ctx context.Context, input json.RawMessage) (string, error)) *ToolFunc {
	return &ToolFunc{name: name, description: description, schema: schema, fn: fn}
}

func (t *ToolFunc) Name() string            { return t.name }
func (t *ToolFunc) Description() string     { return t.description }
func (t *ToolFunc) Schema() json.RawMessage { return t.schema }

func (t *ToolFunc) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	return t.fn(ctx, input)
}

// ToolAccess represents a tool declaration from an agent's frontmatter.
type ToolAccess struct {
	Name string
}

// UnmarshalYAML handles string tool names in agent frontmatter.
func (t *ToolAccess) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("tool declaration must be a string, got %v", value.Kind)
	}
	t.Name = value.Value
	return nil
}
