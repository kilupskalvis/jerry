// Agent orchestrates a tool-using LLM interaction loop. It sends messages to a
// Provider, dispatches tool calls, feeds results back, and repeats until the
// provider returns a final text response.

package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/tool"
)

const defaultMaxTurns = 10

// ErrMaxTurns is returned when the agent exceeds its configured turn limit.
var ErrMaxTurns = errors.New("agent exceeded maximum turns")

// Agent orchestrates a tool-using LLM interaction loop.
type Agent struct {
	provider     llm.Provider
	tools        []tool.Tool
	systemPrompt string
	model        string
	maxTurns     int
	logger       *slog.Logger
}

// Option configures an Agent.
type Option func(*Agent)

// WithTools sets the tools available to the agent.
func WithTools(tools ...tool.Tool) Option {
	return func(a *Agent) { a.tools = tools }
}

// WithSystemPrompt sets the system-level instruction.
func WithSystemPrompt(prompt string) Option {
	return func(a *Agent) { a.systemPrompt = prompt }
}

// WithModel sets the LLM model identifier.
func WithModel(model string) Option {
	return func(a *Agent) { a.model = model }
}

// WithMaxTurns sets the maximum number of LLM round-trips before aborting.
func WithMaxTurns(n int) Option {
	return func(a *Agent) { a.maxTurns = n }
}

// WithLogger sets a structured logger for the agent.
func WithLogger(logger *slog.Logger) Option {
	return func(a *Agent) { a.logger = logger }
}

// NewAgent creates an Agent with the given provider and options.
func NewAgent(provider llm.Provider, opts ...Option) *Agent {
	a := &Agent{
		provider: provider,
		maxTurns: defaultMaxTurns,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Run executes the agentic loop: sends the input to the LLM, dispatches any
// tool calls, feeds results back, and repeats until the LLM returns a final
// text response or the turn limit is reached.
func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	messages := []llm.Message{{Role: llm.RoleUser, Content: input}}
	toolDefs := toolsToDefinitions(a.tools)
	toolMap := a.buildToolMap()

	for turn := range a.maxTurns {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		resp, err := a.provider.Complete(ctx, llm.CompleteParams{
			Model:        a.model,
			SystemPrompt: a.systemPrompt,
			Messages:     messages,
			Tools:        toolDefs,
		})
		if err != nil {
			return "", fmt.Errorf("provider complete (turn %d): %w", turn, err)
		}

		messages = append(messages, resp.Message)
		a.logger.Info("agent turn",
			"turn", turn,
			"stop_reason", resp.StopReason,
			"tool_calls", len(resp.Message.ToolCalls),
			"input_tokens", resp.Usage.InputTokens,
			"output_tokens", resp.Usage.OutputTokens,
			"cache_creation", resp.Usage.CacheCreationTokens,
			"cache_read", resp.Usage.CacheReadTokens,
		)

		if resp.StopReason != llm.StopReasonToolUse || len(resp.Message.ToolCalls) == 0 {
			return resp.Message.Content, nil
		}

		results := a.executeTools(ctx, resp.Message.ToolCalls, toolMap)
		for _, r := range results {
			messages = append(messages, r.ToMessage())
		}
	}

	return "", ErrMaxTurns
}

func toolsToDefinitions(tools []tool.Tool) []llm.ToolDefinition {
	defs := make([]llm.ToolDefinition, len(tools))
	for i, t := range tools {
		defs[i] = llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		}
	}
	return defs
}

func (a *Agent) buildToolMap() map[string]tool.Tool {
	m := make(map[string]tool.Tool, len(a.tools))
	for _, t := range a.tools {
		m[t.Name()] = t
	}
	return m
}

func (a *Agent) executeTools(ctx context.Context, calls []llm.ToolCall, toolMap map[string]tool.Tool) []llm.ToolResult {
	results := make([]llm.ToolResult, 0, len(calls))
	for _, call := range calls {
		t, ok := toolMap[call.Name]
		if !ok {
			results = append(results, llm.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("unknown tool: %s", call.Name),
				IsError: true,
			})
			a.logger.Warn("unknown tool requested", "tool", call.Name)
			continue
		}

		output, err := t.Execute(ctx, call.Input)
		if err != nil {
			results = append(results, llm.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("tool execution failed: %v", err),
				IsError: true,
			})
			a.logger.Warn("tool execution failed", "tool", call.Name, "error", err)
			continue
		}

		results = append(results, llm.ToolResult{
			CallID:  call.ID,
			Content: output,
		})
	}
	return results
}
