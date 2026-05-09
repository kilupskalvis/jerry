// Provider-neutral conversation primitives for the agent harness.

package llm

import "encoding/json"

// Role identifies the sender of a message in the conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single entry in the conversation between user, assistant, and tools.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a single tool invocation requested by the LLM.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult holds the outcome of executing a tool call.
type ToolResult struct {
	CallID  string
	Content string
	IsError bool
}

// ToMessage converts a ToolResult into a Message suitable for the conversation history.
func (r ToolResult) ToMessage() Message {
	content := r.Content
	if r.IsError {
		content = "ERROR: " + content
	}
	return Message{
		Role:       RoleTool,
		Content:    content,
		ToolCallID: r.CallID,
	}
}
