// Agentic loop: send messages to an LLM, execute tool calls, repeat until done.

package agent

import (
	"context"
	"encoding/json"
	"fmt"

	motifErrors "github.com/kilupskalvis/motif/internal/errors"
	"github.com/kilupskalvis/motif/internal/llm"
	"github.com/kilupskalvis/motif/internal/output"
)

// MaxRetryIterations is the maximum additional iterations allowed during
// output format retry (RunRetry).
const MaxRetryIterations = 5

// Loop runs an autonomous agent to completion using the agentic loop pattern:
// send messages to the LLM, execute tool calls, feed results back, repeat
// until the LLM produces a final text response with no tool calls.
type Loop struct {
	client  llm.Client
	printer *output.Printer
}

// NewLoop creates a new agentic loop with the given LLM client.
func NewLoop(client llm.Client, printer *output.Printer) *Loop {
	return &Loop{
		client:  client,
		printer: printer,
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
//
// The system message is constructed from the agent's instructions, pipeline context,
// and output schema instructions. The loop starts with a minimal "Begin your task."
// user message (required by the Anthropic API).
func (l *Loop) Run(
	loopCtx context.Context,
	config AgentConfig,
	toolDefs []llm.ToolDef,
	dispatch func(context.Context, llm.ToolCall) (string, error),
	pipelineContext map[string]any,
) (*LoopResult, error) {
	systemMessage := buildSystemMessage(config, pipelineContext)

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Begin your task."},
	}

	result := &LoopResult{
		SystemMessage: systemMessage,
	}

	for result.Iterations < config.MaxIterations {
		// Check for cancellation (timeout, Ctrl+C).
		if loopCtx.Err() != nil {
			return nil, loopCtx.Err()
		}

		response, err := l.client.Send(loopCtx, systemMessage, messages, toolDefs)
		if err != nil {
			return nil, err
		}

		// Accumulate token usage.
		result.TotalUsage.InputTokens += response.Usage.InputTokens
		result.TotalUsage.OutputTokens += response.Usage.OutputTokens
		result.Iterations++

		// If the model made tool calls, execute them and continue.
		if len(response.ToolCalls) > 0 {
			// Append assistant message (with tool calls) to history.
			messages = append(messages, llm.Message{
				Role:      llm.RoleAssistant,
				Content:   response.Content,
				ToolCalls: response.ToolCalls,
			})

			// Execute each tool call sequentially.
			for _, call := range response.ToolCalls {
				result.ToolCalls++
				l.printer.Info(fmt.Sprintf("      tool: %s", call.Name))

				toolResult, toolErr := dispatch(loopCtx, call)
				if toolErr != nil {
					// Tool execution error — feed error back to agent.
					toolResult = fmt.Sprintf("Error executing %s: %s", call.Name, toolErr.Error())
				}

				messages = append(messages, llm.Message{
					Role:    llm.RoleTool,
					ToolID:  call.ID,
					Content: toolResult,
				})
			}

			continue
		}

		// No tool calls — agent is done. response.Content is the final output.
		result.RawOutput = response.Content
		result.Messages = messages
		return result, nil
	}

	// Max iterations reached.
	result.Messages = messages
	return result, motifErrors.New(motifErrors.CodeAgentMaxIterations,
		fmt.Sprintf("agent %q reached max iterations (%d)", config.Name, config.MaxIterations))
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
	// Continue from the previous conversation.
	messages := make([]llm.Message, len(previous.Messages))
	copy(messages, previous.Messages)

	// Append the agent's raw output as an assistant message.
	messages = append(messages, llm.Message{
		Role:    llm.RoleAssistant,
		Content: previous.RawOutput,
	})

	// Append the correction prompt.
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: correctionMessage,
	})

	result := &LoopResult{
		SystemMessage: previous.SystemMessage,
		TotalUsage:    previous.TotalUsage,
		ToolCalls:     previous.ToolCalls,
	}

	for result.Iterations < MaxRetryIterations {
		if loopCtx.Err() != nil {
			return nil, loopCtx.Err()
		}

		response, err := l.client.Send(loopCtx, previous.SystemMessage, messages, toolDefs)
		if err != nil {
			return nil, err
		}

		result.TotalUsage.InputTokens += response.Usage.InputTokens
		result.TotalUsage.OutputTokens += response.Usage.OutputTokens
		result.Iterations++

		if len(response.ToolCalls) > 0 {
			messages = append(messages, llm.Message{
				Role:      llm.RoleAssistant,
				Content:   response.Content,
				ToolCalls: response.ToolCalls,
			})

			for _, call := range response.ToolCalls {
				result.ToolCalls++

				toolResult, toolErr := dispatch(loopCtx, call)
				if toolErr != nil {
					toolResult = fmt.Sprintf("Error executing %s: %s", call.Name, toolErr.Error())
				}

				messages = append(messages, llm.Message{
					Role:    llm.RoleTool,
					ToolID:  call.ID,
					Content: toolResult,
				})
			}

			continue
		}

		result.RawOutput = response.Content
		result.Messages = messages
		return result, nil
	}

	result.Messages = messages
	return result, fmt.Errorf("agent did not produce valid output after %d retry iterations", MaxRetryIterations)
}

// buildSystemMessage constructs the system prompt from the agent's instructions,
// pipeline context, and output schema.
func buildSystemMessage(config AgentConfig, pipelineContext map[string]any) string {
	result := config.Instructions

	if len(pipelineContext) > 0 {
		contextJSON, err := json.MarshalIndent(pipelineContext, "", "  ")
		if err == nil {
			result += "\n\n## Pipeline Context\n\n```json\n" + string(contextJSON) + "\n```"
		}
	}

	if config.OutputSchema != nil {
		schemaJSON, err := json.MarshalIndent(config.OutputSchema, "", "  ")
		if err == nil {
			result += "\n\n## Output Format\n\n" +
				"When you have completed your task, respond with a JSON object " +
				"matching this schema (no markdown fences, raw JSON only):\n\n```json\n" +
				string(schemaJSON) + "\n```"
		}
	}

	return result
}
