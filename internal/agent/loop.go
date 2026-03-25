// Agentic loop: send messages to an LLM, execute tool calls, repeat until done.

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	jerryErrors "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/state"
)

// MaxRetryIterations is the maximum additional iterations allowed during
// output format retry (RunRetry).
const MaxRetryIterations = 5

// Loop runs an autonomous agent to completion using the agentic loop pattern:
// send messages to the LLM, execute tool calls, feed results back, repeat
// until the LLM produces a final text response with no tool calls.
type Loop struct {
	client    llm.Client
	compactor *llm.Compactor
	printer   *output.Printer
}

// NewLoop creates a new agentic loop with the given LLM client.
// compactor may be nil — compaction is skipped if not provided.
func NewLoop(client llm.Client, compactor *llm.Compactor, printer *output.Printer) *Loop {
	return &Loop{
		client:    client,
		compactor: compactor,
		printer:   printer,
	}
}

// LoopResult holds the outcome of an agentic loop execution.
type LoopResult struct {
	// RawOutput is the agent's final text response (the last response with
	// no tool calls).
	RawOutput string

	// Messages is the full conversation history from the loop.
	// Needed by RunRetry to continue the conversation with a correction prompt.
	Messages []llm.Message

	// SystemMessage is the system prompt used for this run.
	// Needed by RunRetry to preserve the same system context.
	SystemMessage string

	// Iterations is how many loop cycles ran.
	Iterations int

	// TotalUsage is the accumulated token usage across all LLM calls.
	TotalUsage llm.TokenUsage

	// ToolCalls is the total number of tool calls made.
	ToolCalls int
}

// Run executes the agentic loop until the agent completes or a limit is reached.
func (l *Loop) Run(
	loopCtx context.Context,
	agentCfg AgentConfig,
	toolDefs []llm.ToolDef,
	dispatch func(context.Context, llm.ToolCall) (string, error),
	pipelineContext map[string]any,
) (*LoopResult, error) {
	systemMessage := buildSystemMessage(agentCfg, pipelineContext)

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Begin your task."},
	}

	result := &LoopResult{
		SystemMessage: systemMessage,
	}

	compactionAttempts := 0

	for result.Iterations < agentCfg.MaxIterations {
		if loopCtx.Err() != nil {
			return nil, loopCtx.Err()
		}

		llmStart := time.Now()
		response, sendErr := l.client.Send(loopCtx, systemMessage, messages, toolDefs)

		// Reactive compaction: context too long.
		if sendErr != nil && llm.IsContextTooLong(sendErr) && l.compactor != nil {
			compacted, compErr := l.reactiveCompact(loopCtx, systemMessage, messages, &compactionAttempts, result)
			if compErr != nil {
				return nil, compErr
			}
			messages = compacted
			continue
		}

		if sendErr != nil {
			return nil, sendErr
		}

		result.TotalUsage.InputTokens += response.Usage.InputTokens
		result.TotalUsage.OutputTokens += response.Usage.OutputTokens
		result.Iterations++

		// Log LLM call.
		if lw := state.LogWriterFrom(loopCtx); lw != nil {
			toolNames := make([]string, len(response.ToolCalls))
			for i, tc := range response.ToolCalls {
				toolNames[i] = tc.Name
			}
			lw.Log(state.LogLLMCall, state.StepNameFrom(loopCtx), state.LLMCallData{
				Iteration:          result.Iterations,
				Model:              agentCfg.Model,
				TokensInput:        response.Usage.InputTokens,
				TokensOutput:       response.Usage.OutputTokens,
				DurationMs:         time.Since(llmStart).Milliseconds(),
				StopReason:         response.StopReason,
				ToolCallsRequested: toolNames,
			})
		}

		// Proactive compaction: approaching context window limit.
		messages = l.proactiveCompact(loopCtx, agentCfg, systemMessage, messages, response, result)

		if len(response.ToolCalls) > 0 {
			messages = l.executeToolCalls(loopCtx, messages, response, dispatch, result)
			continue
		}

		// No tool calls — agent is done.
		result.RawOutput = response.Content
		result.Messages = messages
		return result, nil
	}

	result.Messages = messages
	return result, jerryErrors.New(jerryErrors.CodeAgentMaxIterations,
		fmt.Sprintf("agent %q reached max iterations (%d)", agentCfg.Name, agentCfg.MaxIterations))
}

// RunRetry sends a correction message and runs additional iterations.
// Used when the agent's output doesn't match the expected JSON schema.
func (l *Loop) RunRetry(
	loopCtx context.Context,
	previous *LoopResult,
	toolDefs []llm.ToolDef,
	dispatch func(context.Context, llm.ToolCall) (string, error),
	correctionMessage string,
) (*LoopResult, error) {
	messages := make([]llm.Message, len(previous.Messages))
	copy(messages, previous.Messages)

	messages = append(messages, llm.Message{
		Role:    llm.RoleAssistant,
		Content: previous.RawOutput,
	})
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: correctionMessage,
	})

	result := &LoopResult{
		SystemMessage: previous.SystemMessage,
		TotalUsage:    previous.TotalUsage,
		ToolCalls:     previous.ToolCalls,
	}

	compactionAttempts := 0

	for result.Iterations < MaxRetryIterations {
		if loopCtx.Err() != nil {
			return nil, loopCtx.Err()
		}

		response, sendErr := l.client.Send(loopCtx, previous.SystemMessage, messages, toolDefs)

		// Reactive compaction during retry.
		if sendErr != nil && llm.IsContextTooLong(sendErr) && l.compactor != nil {
			compacted, compErr := l.reactiveCompact(loopCtx, previous.SystemMessage, messages, &compactionAttempts, result)
			if compErr != nil {
				return nil, compErr
			}
			messages = compacted
			continue
		}

		if sendErr != nil {
			return nil, sendErr
		}

		result.TotalUsage.InputTokens += response.Usage.InputTokens
		result.TotalUsage.OutputTokens += response.Usage.OutputTokens
		result.Iterations++

		if len(response.ToolCalls) > 0 {
			messages = l.executeToolCalls(loopCtx, messages, response, dispatch, result)
			continue
		}

		result.RawOutput = response.Content
		result.Messages = messages
		return result, nil
	}

	result.Messages = messages
	return result, fmt.Errorf("agent did not produce valid output after %d retry iterations", MaxRetryIterations)
}

// reactiveCompact handles compaction when the context window is exceeded.
func (l *Loop) reactiveCompact(
	loopCtx context.Context,
	systemMessage string,
	messages []llm.Message,
	attempts *int,
	result *LoopResult,
) ([]llm.Message, error) {
	*attempts++
	if *attempts > llm.MaxCompactionAttempts {
		return nil, fmt.Errorf("context too long after %d compaction attempts", llm.MaxCompactionAttempts)
	}

	keepRecent := llm.DefaultKeepRecent - (*attempts - 1)
	if keepRecent < 1 {
		keepRecent = 1
	}

	compactionResult, compErr := l.compactor.Compact(loopCtx, systemMessage, messages, keepRecent)
	if compErr != nil {
		return nil, fmt.Errorf("context too long and compaction failed: %w", compErr)
	}

	result.TotalUsage.InputTokens += compactionResult.Usage.InputTokens
	result.TotalUsage.OutputTokens += compactionResult.Usage.OutputTokens
	l.printer.Info("  [compacted conversation after context overflow, attempt %d]", *attempts)

	return compactionResult.CompactedMessages, nil
}

// proactiveCompact compacts if we're approaching the context window limit.
func (l *Loop) proactiveCompact(
	loopCtx context.Context,
	agentCfg AgentConfig,
	systemMessage string,
	messages []llm.Message,
	response *llm.Response,
	result *LoopResult,
) []llm.Message {
	if agentCfg.ContextWindow <= 0 || l.compactor == nil {
		return messages
	}

	threshold := int(float64(agentCfg.ContextWindow) * llm.ProactiveCompactionThreshold)
	if response.Usage.InputTokens <= threshold {
		return messages
	}

	compactionResult, compErr := l.compactor.Compact(loopCtx, systemMessage, messages, llm.DefaultKeepRecent)
	if compErr != nil {
		l.printer.Warning("proactive compaction failed: %s", compErr)
		return messages
	}

	result.TotalUsage.InputTokens += compactionResult.Usage.InputTokens
	result.TotalUsage.OutputTokens += compactionResult.Usage.OutputTokens
	l.printer.Info("  [compacted conversation — evicted %d messages]", compactionResult.EvictedCount)

	return compactionResult.CompactedMessages
}

// executeToolCalls runs all tool calls from a response and appends results to history.
func (l *Loop) executeToolCalls(
	loopCtx context.Context,
	messages []llm.Message,
	response *llm.Response,
	dispatch func(context.Context, llm.ToolCall) (string, error),
	result *LoopResult,
) []llm.Message {
	messages = append(messages, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   response.Content,
		ToolCalls: response.ToolCalls,
	})

	lw := state.LogWriterFrom(loopCtx)
	for _, call := range response.ToolCalls {
		result.ToolCalls++

		toolStart := time.Now()
		toolResult, toolErr := dispatch(loopCtx, call)
		toolDuration := time.Since(toolStart)
		success := toolErr == nil

		if toolErr != nil {
			toolResult = fmt.Sprintf("Error executing %s: %s", call.Name, toolErr.Error())
		}

		summary := summarizeToolCall(call, toolResult)
		l.printer.ToolProgress(result.Iterations, call.Name, summary)

		if lw != nil {
			lw.Log(state.LogToolCall, state.StepNameFrom(loopCtx), state.ToolCallData{
				Iteration:       result.Iterations,
				Tool:            call.Name,
				Summary:         summary,
				Arguments:       call.Arguments,
				DurationMs:      toolDuration.Milliseconds(),
				ResultSizeBytes: len(toolResult),
				Success:         success,
			})
		}

		messages = append(messages, llm.Message{
			Role:    llm.RoleTool,
			ToolID:  call.ID,
			Content: toolResult,
		})
	}

	return messages
}

// summarizeToolCall produces a short summary of a tool call for progress output.
func summarizeToolCall(call llm.ToolCall, result string) string {
	switch call.Name {
	case "read_file", "write_file":
		if path, ok := call.Arguments["path"].(string); ok {
			return path
		}
	case "glob":
		if pattern, ok := call.Arguments["pattern"].(string); ok {
			return pattern
		}
	case "search_codebase":
		if query, ok := call.Arguments["query"].(string); ok {
			return fmt.Sprintf("%q", query)
		}
	case "run_command":
		if cmd, ok := call.Arguments["command"].(string); ok {
			if len(cmd) > 60 {
				cmd = cmd[:60] + "..."
			}
			return cmd
		}
	case "list_directory":
		if path, ok := call.Arguments["path"].(string); ok {
			return path
		}
	}
	return ""
}

// buildSystemMessage constructs the system prompt from the agent's instructions,
// pipeline context, and output schema.
func buildSystemMessage(agentCfg AgentConfig, pipelineContext map[string]any) string {
	result := agentCfg.Instructions

	if len(pipelineContext) > 0 {
		contextJSON, err := json.MarshalIndent(pipelineContext, "", "  ")
		if err == nil {
			result += "\n\n## Pipeline Context\n\n```json\n" + string(contextJSON) + "\n```"
		}
	}

	if agentCfg.OutputSchema != nil {
		schemaJSON, err := json.MarshalIndent(agentCfg.OutputSchema, "", "  ")
		if err == nil {
			result += "\n\n## Output Format\n\n" +
				"When you have completed your task, respond with a JSON object " +
				"matching this schema (no markdown fences, raw JSON only):\n\n```json\n" +
				string(schemaJSON) + "\n```"
		}
	}

	return result
}
