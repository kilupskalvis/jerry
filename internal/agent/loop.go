package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/state"
)

// MaxRetryIterations is the maximum iterations for output format retries.
const MaxRetryIterations = 5

// Loop runs an autonomous agent to completion.
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
	RawOutput     string
	Messages      []llm.Message
	SystemMessage string
	Iterations    int
	TotalUsage    llm.TokenUsage
	ToolCalls     int
}

// Run executes the agentic loop until the agent completes or a limit is reached.
func (l *Loop) Run(
	ctx context.Context,
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
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		llmStart := time.Now()
		response, sendErr := l.client.Send(ctx, systemMessage, messages, toolDefs)

		// Reactive compaction: context too long.
		if sendErr != nil && llm.IsContextTooLong(sendErr) && l.compactor != nil {
			compacted, compErr := l.reactiveCompact(ctx, systemMessage, messages, &compactionAttempts, result)
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
		if lw := state.LogWriterFrom(ctx); lw != nil {
			toolNames := make([]string, len(response.ToolCalls))
			for i, tc := range response.ToolCalls {
				toolNames[i] = tc.Name
			}
			lw.Log(state.LogLLMCall, state.StepNameFrom(ctx), state.LLMCallData{
				Iteration:          result.Iterations,
				Model:              agentCfg.Model,
				TokensInput:        response.Usage.InputTokens,
				TokensOutput:       response.Usage.OutputTokens,
				DurationMs:         time.Since(llmStart).Milliseconds(),
				StopReason:         response.StopReason,
				ToolCallsRequested: toolNames,
			})
		}

		if len(response.ToolCalls) > 0 {
			messages = l.executeToolCalls(ctx, messages, response, dispatch, result)
			continue
		}

		// No tool calls — agent is done.
		result.RawOutput = response.Content
		result.Messages = messages
		return result, nil
	}

	result.Messages = messages
	return result, jerrerr.New(jerrerr.CodeAgentMaxIterations,
		fmt.Sprintf("agent %q reached max iterations (%d)", agentCfg.Name, agentCfg.MaxIterations))
}

// RunRetry sends a correction message and runs additional iterations.
// Used when the agent's output doesn't match the expected JSON schema.
func (l *Loop) RunRetry(
	ctx context.Context,
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
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		response, sendErr := l.client.Send(ctx, previous.SystemMessage, messages, toolDefs)

		// Reactive compaction during retry.
		if sendErr != nil && llm.IsContextTooLong(sendErr) && l.compactor != nil {
			compacted, compErr := l.reactiveCompact(ctx, previous.SystemMessage, messages, &compactionAttempts, result)
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
			messages = l.executeToolCalls(ctx, messages, response, dispatch, result)
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
	ctx context.Context,
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

	compactionResult, compErr := l.compactor.Compact(ctx, systemMessage, messages, keepRecent)
	if compErr != nil {
		return nil, fmt.Errorf("context too long and compaction failed: %w", compErr)
	}

	result.TotalUsage.InputTokens += compactionResult.Usage.InputTokens
	result.TotalUsage.OutputTokens += compactionResult.Usage.OutputTokens
	l.printer.Info("  [compacted conversation after context overflow, attempt %d]", *attempts)

	return compactionResult.CompactedMessages, nil
}

// executeToolCalls runs all tool calls from a response and appends results to history.
func (l *Loop) executeToolCalls(
	ctx context.Context,
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

	lw := state.LogWriterFrom(ctx)
	for _, call := range response.ToolCalls {
		result.ToolCalls++

		toolStart := time.Now()
		toolResult, toolErr := dispatch(ctx, call)
		toolDuration := time.Since(toolStart)
		success := toolErr == nil

		if toolErr != nil {
			toolResult = fmt.Sprintf("Error executing %s: %s", call.Name, toolErr.Error())
		}

		summary := summarizeToolCall(call, toolResult)
		l.printer.ToolProgress(result.Iterations, call.Name, summary)

		if lw != nil {
			lw.Log(state.LogToolCall, state.StepNameFrom(ctx), state.ToolCallData{
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
