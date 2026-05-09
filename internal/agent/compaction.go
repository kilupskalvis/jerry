// Compactor manages conversation history compaction by summarizing older
// messages and preserving recent ones.

package agent

import (
	"context"
	"fmt"
	"strings"
)

const (
	// DefaultKeepRecent is the number of recent assistant turns to preserve
	// during compaction. A turn = one assistant message + its tool results.
	DefaultKeepRecent = 5

	// MaxCompactionAttempts limits compaction-then-retry cycles before giving up.
	MaxCompactionAttempts = 3

	maxToolResultPreview = 500
)

// CompactionResult holds the output of a compaction operation.
type CompactionResult struct {
	CompactedMessages []Message
	Summary           string
	EvictedCount      int
	Usage             Usage
}

// Compactor shortens conversation history when the context window is exceeded.
type Compactor struct {
	provider Provider
}

// NewCompactor creates a compactor that uses the given provider for summarization.
func NewCompactor(provider Provider) *Compactor {
	return &Compactor{provider: provider}
}

// Compact shortens conversation history by summarizing older messages.
// It preserves the most recent keepRecent assistant turns and summarizes
// everything before that split point.
func (c *Compactor) Compact(
	ctx context.Context,
	systemPrompt string,
	messages []Message,
	keepRecent int,
) (*CompactionResult, error) {
	splitIndex := findSplitPoint(messages, keepRecent)

	oldMessages := messages[:splitIndex]
	recentMessages := messages[splitIndex:]

	if len(oldMessages) == 0 {
		return nil, fmt.Errorf("cannot compact further — conversation is already minimal")
	}

	summaryRequest := "Summarize the following conversation between an AI agent " +
		"and its tools. Preserve: key decisions made, important findings, " +
		"files read or modified, errors encountered, and any information " +
		"the agent will need to continue its task. Be concise but complete.\n\n" +
		formatMessagesForSummary(oldMessages)

	resp, err := c.provider.Complete(ctx, CompleteParams{
		SystemPrompt: "You are a conversation summarizer. Output only the summary.",
		Messages:     []Message{{Role: RoleUser, Content: summaryRequest}},
	})
	if err != nil {
		return nil, fmt.Errorf("compaction summarization failed: %w", err)
	}

	compacted := make([]Message, 0, 1+len(recentMessages))
	compacted = append(compacted, Message{
		Role:    RoleUser,
		Content: "[Conversation summary]\n" + resp.Message.Content,
	})
	compacted = append(compacted, recentMessages...)

	return &CompactionResult{
		CompactedMessages: compacted,
		Summary:           resp.Message.Content,
		EvictedCount:      len(oldMessages),
		Usage:             resp.Usage,
	}, nil
}

// findSplitPoint walks backward through messages counting assistant turns.
func findSplitPoint(messages []Message, keepRecent int) int {
	turnsFound := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleAssistant {
			turnsFound++
			if turnsFound == keepRecent {
				return i
			}
		}
	}
	return 0
}

func formatMessagesForSummary(messages []Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case RoleAssistant:
			sb.WriteString("ASSISTANT: ")
			if msg.Content != "" {
				sb.WriteString(msg.Content)
			}
			for _, tc := range msg.ToolCalls {
				fmt.Fprintf(&sb, "\n  [called tool: %s]", tc.Name)
			}
		case RoleTool:
			fmt.Fprintf(&sb, "TOOL RESULT (%s): ", msg.ToolCallID)
			content := msg.Content
			if len(content) > maxToolResultPreview {
				content = content[:maxToolResultPreview] + "... [truncated]"
			}
			sb.WriteString(content)
		case RoleUser:
			sb.WriteString("USER: ")
			sb.WriteString(msg.Content)
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}
