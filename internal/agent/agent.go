// Agent orchestrates a tool-using LLM interaction loop. It sends messages to a
// Provider, dispatches tool calls, feeds results back, and repeats until the
// provider returns a final text response.

package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/permissions"
	"github.com/kilupskalvis/jerry/internal/tool"
)

const defaultMaxTurns = 10

// ErrMaxTurns is returned when the agent exceeds its configured turn limit.
var ErrMaxTurns = errors.New("agent exceeded maximum turns")

// RunResult holds the output and accumulated stats from an agent execution.
type RunResult struct {
	Output    string
	Usage     llm.Usage
	Turns     int
	ToolCalls int
}

// EventHandler receives detailed execution events from the agent loop.
type EventHandler struct {
	OnTurn       func(turn int, stopReason string, toolCalls, inputTokens, outputTokens, cacheCreation, cacheRead int)
	OnToolCall   func(name, args string)
	OnToolResult func(name, result string, isError bool)
	OnResponse   func(text string)
}

// Agent orchestrates a tool-using LLM interaction loop.
type Agent struct {
	provider     llm.Provider
	tools        []tool.Tool
	systemPrompt string
	model        string
	maxTurns     int
	temperature  *float64
	logger       *slog.Logger
	onToolCall   func(toolName string)
	events       *EventHandler
	checker      permissions.Checker
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

// WithTemperature sets the LLM sampling temperature.
func WithTemperature(t *float64) Option {
	return func(a *Agent) { a.temperature = t }
}

// WithLogger sets a structured logger for the agent.
func WithLogger(logger *slog.Logger) Option {
	return func(a *Agent) { a.logger = logger }
}

// WithOnToolCall sets a callback invoked before each tool execution.
func WithOnToolCall(fn func(toolName string)) Option {
	return func(a *Agent) { a.onToolCall = fn }
}

// WithEventHandler sets detailed event callbacks for the agent loop.
func WithEventHandler(h *EventHandler) Option {
	return func(a *Agent) { a.events = h }
}

// WithChecker sets a permission checker for tool call enforcement.
func WithChecker(c permissions.Checker) Option {
	return func(a *Agent) { a.checker = c }
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
func (a *Agent) Run(ctx context.Context, input string) (*RunResult, error) {
	messages := []llm.Message{{Role: llm.RoleUser, Content: input}}
	toolDefs := toolsToDefinitions(a.tools)
	toolMap := a.buildToolMap()

	var total llm.Usage
	var totalToolCalls int

	for turn := range a.maxTurns {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := a.provider.Complete(ctx, llm.CompleteParams{
			Model:        a.model,
			SystemPrompt: a.systemPrompt,
			Messages:     messages,
			Tools:        toolDefs,
			Temperature:  a.temperature,
		})
		if err != nil {
			return nil, fmt.Errorf("provider complete (turn %d): %w", turn, err)
		}

		total.InputTokens += resp.Usage.InputTokens
		total.OutputTokens += resp.Usage.OutputTokens
		total.CacheCreationTokens += resp.Usage.CacheCreationTokens
		total.CacheReadTokens += resp.Usage.CacheReadTokens
		totalToolCalls += len(resp.Message.ToolCalls)

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
		if a.events != nil && a.events.OnTurn != nil {
			a.events.OnTurn(turn, string(resp.StopReason),
				len(resp.Message.ToolCalls), resp.Usage.InputTokens, resp.Usage.OutputTokens,
				resp.Usage.CacheCreationTokens, resp.Usage.CacheReadTokens)
		}

		if resp.StopReason != llm.StopReasonToolUse || len(resp.Message.ToolCalls) == 0 {
			if a.events != nil && a.events.OnResponse != nil {
				a.events.OnResponse(resp.Message.Content)
			}
			return &RunResult{
				Output:    resp.Message.Content,
				Usage:     total,
				Turns:     turn + 1,
				ToolCalls: totalToolCalls,
			}, nil
		}

		results := a.executeTools(ctx, resp.Message.ToolCalls, toolMap)
		for _, r := range results {
			messages = append(messages, r.ToMessage())
		}
	}

	return nil, ErrMaxTurns
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
	results := make([]llm.ToolResult, len(calls))
	var wg sync.WaitGroup

	for i, call := range calls {
		t, ok := toolMap[call.Name]
		if !ok {
			results[i] = llm.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("unknown tool: %s", call.Name),
				IsError: true,
			}
			a.logger.Warn("unknown tool requested", "tool", call.Name)
			continue
		}

		if a.checker != nil {
			if denial := a.checker.Check(call.Name, call.Input); denial != nil {
				msg := fmt.Sprintf("Permission denied: %q blocked by guardrail.\nDenied pattern: %q (source: %s)",
					denial.Input, denial.Pattern, denial.Source)
				results[i] = llm.ToolResult{
					CallID:  call.ID,
					Content: msg,
					IsError: true,
				}
				a.emitToolResult(call.Name, msg, true)
				a.logger.Warn("tool call denied by guardrail",
					"tool", call.Name, "pattern", denial.Pattern, "source", denial.Source)
				continue
			}
		}

		wg.Add(1)
		go func(idx int, call llm.ToolCall, t tool.Tool) {
			defer wg.Done()

			a.emitToolCall(call.Name, string(call.Input))

			output, err := t.Execute(ctx, call.Input)
			if err != nil {
				errMsg := fmt.Sprintf("tool execution failed: %v", err)
				results[idx] = llm.ToolResult{
					CallID:  call.ID,
					Content: errMsg,
					IsError: true,
				}
				a.emitToolResult(call.Name, errMsg, true)
				a.logger.Warn("tool execution failed", "tool", call.Name, "error", err)
				return
			}

			a.emitToolResult(call.Name, output, false)
			results[idx] = llm.ToolResult{
				CallID:  call.ID,
				Content: output,
			}
		}(i, call, t)
	}

	wg.Wait()
	return results
}

func (a *Agent) emitToolCall(name, args string) {
	if a.onToolCall != nil {
		a.onToolCall(name)
	}
	if a.events != nil && a.events.OnToolCall != nil {
		a.events.OnToolCall(name, args)
	}
}

func (a *Agent) emitToolResult(name, result string, isError bool) {
	if a.events != nil && a.events.OnToolResult != nil {
		a.events.OnToolResult(name, result, isError)
	}
}
