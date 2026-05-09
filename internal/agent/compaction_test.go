package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kilupskalvis/jerry/internal/agent"
)

type mockSummarizer struct {
	response string
	err      error
	calls    int
}

func (m *mockSummarizer) Complete(_ context.Context, _ agent.CompleteParams) (*agent.CompleteResponse, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &agent.CompleteResponse{
		Message:    agent.Message{Role: agent.RoleAssistant, Content: m.response},
		StopReason: agent.StopReasonEndTurn,
		Usage:      agent.Usage{InputTokens: 100, OutputTokens: 50},
	}, nil
}

func buildTestMessages(count int) []agent.Message {
	msgs := make([]agent.Message, 0, count)
	for i := range count {
		switch i % 3 {
		case 0:
			msgs = append(msgs, agent.Message{
				Role:    agent.RoleAssistant,
				Content: fmt.Sprintf("assistant message %d", i),
				ToolCalls: []agent.ToolCall{
					{ID: fmt.Sprintf("call_%d", i), Name: "read_file", Input: json.RawMessage(`{"path":"test.go"}`)},
				},
			})
		case 1:
			msgs = append(msgs, agent.Message{
				Role:       agent.RoleTool,
				ToolCallID: fmt.Sprintf("call_%d", i-1),
				Content:    fmt.Sprintf("tool result %d", i),
			})
		case 2:
			msgs = append(msgs, agent.Message{
				Role:    agent.RoleUser,
				Content: fmt.Sprintf("user message %d", i),
			})
		}
	}
	return msgs
}

func TestCompact_BasicCompaction(t *testing.T) {
	mock := &mockSummarizer{response: "Summary: agent read files and made decisions."}
	compactor := agent.NewCompactor(mock)

	messages := buildTestMessages(20)
	result, err := compactor.Compact(context.Background(), "system prompt", messages, agent.DefaultKeepRecent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EvictedCount == 0 {
		t.Error("expected some messages to be evicted")
	}
	if len(result.CompactedMessages) == 0 {
		t.Error("expected compacted messages to be non-empty")
	}
	if result.CompactedMessages[0].Role != agent.RoleUser {
		t.Errorf("first compacted message role = %q, want %q", result.CompactedMessages[0].Role, agent.RoleUser)
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestCompact_NothingToCompact(t *testing.T) {
	mock := &mockSummarizer{response: "ignored"}
	compactor := agent.NewCompactor(mock)

	messages := []agent.Message{
		{Role: agent.RoleAssistant, Content: "hello"},
		{Role: agent.RoleUser, Content: "hi"},
	}
	_, err := compactor.Compact(context.Background(), "system", messages, agent.DefaultKeepRecent)
	if err == nil {
		t.Fatal("expected error when nothing to compact")
	}
}

func TestCompact_LLMFailure(t *testing.T) {
	mock := &mockSummarizer{err: fmt.Errorf("API down")}
	compactor := agent.NewCompactor(mock)

	messages := buildTestMessages(20)
	_, err := compactor.Compact(context.Background(), "system", messages, agent.DefaultKeepRecent)
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

func TestCompact_PreservesRecentMessages(t *testing.T) {
	mock := &mockSummarizer{response: "Summary of old messages."}
	compactor := agent.NewCompactor(mock)

	messages := buildTestMessages(30)
	result, err := compactor.Compact(context.Background(), "system", messages, agent.DefaultKeepRecent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.CompactedMessages) < 2 {
		t.Fatalf("expected at least 2 compacted messages, got %d", len(result.CompactedMessages))
	}
}

func TestCompact_UsageTracked(t *testing.T) {
	mock := &mockSummarizer{response: "Summary."}
	compactor := agent.NewCompactor(mock)

	messages := buildTestMessages(20)
	result, err := compactor.Compact(context.Background(), "system", messages, agent.DefaultKeepRecent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Usage.InputTokens == 0 || result.Usage.OutputTokens == 0 {
		t.Error("expected non-zero usage from summarization call")
	}
}
