package agent_test

import (
	"testing"

	"github.com/kilupskalvis/jerry/internal/agent"
)

func TestToolResult_ToMessage(t *testing.T) {
	r := agent.ToolResult{
		CallID:  "call_1",
		Content: "file contents here",
	}
	msg := r.ToMessage()
	if msg.Role != agent.RoleTool {
		t.Errorf("expected RoleTool, got %q", msg.Role)
	}
	if msg.ToolCallID != "call_1" {
		t.Errorf("expected ToolCallID 'call_1', got %q", msg.ToolCallID)
	}
	if msg.Content != "file contents here" {
		t.Errorf("expected content 'file contents here', got %q", msg.Content)
	}
}

func TestToolResult_ToMessage_Error(t *testing.T) {
	r := agent.ToolResult{
		CallID:  "call_2",
		Content: "file not found",
		IsError: true,
	}
	msg := r.ToMessage()
	if msg.Content != "ERROR: file not found" {
		t.Errorf("expected 'ERROR: file not found', got %q", msg.Content)
	}
}

func TestToolResult_ToMessage_EmptyContent(t *testing.T) {
	r := agent.ToolResult{
		CallID: "call_3",
	}
	msg := r.ToMessage()
	if msg.Content != "" {
		t.Errorf("expected empty content, got %q", msg.Content)
	}
	if msg.Role != agent.RoleTool {
		t.Errorf("expected RoleTool, got %q", msg.Role)
	}
}
