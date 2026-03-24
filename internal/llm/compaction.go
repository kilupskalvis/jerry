// Context compaction: summarizes older messages to keep conversations within context limits.

package llm

import (
	"context"
	"fmt"
	"strings"
)

const (
	// DefaultKeepRecent is the number of recent assistant turns to preserve
	// during compaction. A turn = one assistant message + its tool results.
	DefaultKeepRecent = 5

	// ProactiveCompactionThreshold is the fraction of context_window that
	// triggers proactive compaction when exceeded.
	ProactiveCompactionThreshold = 0.80

	// MaxCompactionAttempts limits how many compaction-then-retry cycles
	// are attempted before giving up.
	MaxCompactionAttempts = 3
)

// CompactionResult holds the output of a compaction operation.
type CompactionResult struct {
	CompactedMessages []Message
	Summary           string
	EvictedCount      int
	Usage             TokenUsage
}

// Compactor manages conversation history compaction by summarizing older
// messages and preserving recent ones.
type Compactor struct {
	client Client
}

// NewCompactor creates a compactor that uses the given LLM client for summarization.
func NewCompactor(client Client) *Compactor {
	return &Compactor{client: client}
}

// Compact shortens conversation history by summarizing older messages.
// It preserves the most recent keepRecent assistant turns and summarizes
// everything before that split point.
func (c *Compactor) Compact(
	requestCtx context.Context,
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

	response, sendErr := c.client.Send(requestCtx,
		"You are a conversation summarizer. Output only the summary.",
		[]Message{{Role: RoleUser, Content: summaryRequest}},
		nil)
	if sendErr != nil {
		return nil, fmt.Errorf("compaction summarization failed: %w", sendErr)
	}

	compacted := make([]Message, 0, 1+len(recentMessages))
	compacted = append(compacted, Message{
		Role:    RoleUser,
		Content: "[Conversation summary]\n" + response.Content,
	})
	compacted = append(compacted, recentMessages...)

	return &CompactionResult{
		CompactedMessages: compacted,
		Summary:           response.Content,
		EvictedCount:      len(oldMessages),
		Usage:             response.Usage,
	}, nil
}

// findSplitPoint walks backward through messages counting assistant turns.
// Returns the index where the "recent" portion begins.
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

// formatMessagesForSummary produces a human-readable representation of
// messages suitable for the summarization prompt.
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
			fmt.Fprintf(&sb, "TOOL RESULT (%s): ", msg.ToolID)
			content := msg.Content
			if len(content) > 500 {
				content = content[:500] + "... [truncated]"
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
