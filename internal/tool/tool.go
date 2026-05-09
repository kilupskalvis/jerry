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
	Name        string
	Constraints map[string]any
}

// UnmarshalYAML handles both string ("read_file") and map (read_file: {restrict_to: [src/]}) forms.
func (t *ToolAccess) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		t.Name = value.Value
		return nil
	case yaml.MappingNode:
		if len(value.Content) != 2 {
			return fmt.Errorf("tool access map must have exactly one key, got %d pairs", len(value.Content)/2)
		}
		t.Name = value.Content[0].Value
		var constraints map[string]any
		if err := value.Content[1].Decode(&constraints); err != nil {
			return fmt.Errorf("cannot parse constraints for tool %q: %w", t.Name, err)
		}
		t.Constraints = constraints
		return nil
	default:
		return fmt.Errorf("tool access must be a string or map, got %v", value.Kind)
	}
}
