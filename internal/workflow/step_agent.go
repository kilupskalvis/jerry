package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kilupskalvis/jerry/internal/agent"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/run"
	"github.com/kilupskalvis/jerry/internal/tool"
	"github.com/kilupskalvis/jerry/internal/trigger"
)

var _ StepExecutor = (*AgentExecutor)(nil)

// AgentExecutor runs agent steps in the workflow.
type AgentExecutor struct {
	loader   *agent.Loader
	registry *tool.Registry
	printer  *output.Printer
	resolver *llm.ProviderResolver
	store    *run.ContextStore

	ProviderOverride llm.Provider
}

func NewAgentExecutor(loader *agent.Loader, registry *tool.Registry, printer *output.Printer, resolver *llm.ProviderResolver) *AgentExecutor {
	return &AgentExecutor{
		loader:   loader,
		registry: registry,
		printer:  printer,
		resolver: resolver,
	}
}

// SetStore sets the context store so agents can access trigger data.
func (e *AgentExecutor) SetStore(store *run.ContextStore) {
	e.store = store
}

func (e *AgentExecutor) CanExecute(step Step) bool {
	return step.Agent != ""
}

func (e *AgentExecutor) Execute(ctx context.Context, step Step, prevOutputs []StepOutput) (*StepOutput, error) {
	start := time.Now()

	agentCfg, err := e.loader.Load(step.Agent)
	if err != nil {
		return nil, err
	}

	provider := e.ProviderOverride
	if provider == nil {
		var provErr error
		provider, provErr = e.resolver.ForModel(agentCfg.Model, agentCfg.Provider)
		if provErr != nil {
			return nil, jerrerr.Wrap(jerrerr.CodeLLMAuthFailed,
				fmt.Sprintf("agent %q", agentCfg.Name), provErr)
		}
	}

	resolvedTools := e.registry.BaseTools()
	optInTools, resolveErr := e.registry.Resolve(agentCfg.Tools)
	if resolveErr != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeToolNotFound,
			fmt.Sprintf("agent %q", agentCfg.Name), resolveErr)
	}
	resolvedTools = append(resolvedTools, optInTools...)

	var triggerData *trigger.TriggerData
	if e.store != nil {
		t := e.store.Trigger()
		triggerData = &t
	}
	systemPrompt := buildSystemPrompt(agentCfg.Instructions, triggerData, prevOutputs)

	var events *agent.EventHandler
	if e.printer != nil {
		events = &agent.EventHandler{
			OnTurn: func(turn int, stopReason string, toolCalls, inputTokens, outputTokens int) {
				e.printer.AgentTurn(turn, stopReason, toolCalls, inputTokens, outputTokens)
			},
			OnToolCall: func(name, args string) {
				e.printer.ToolCallVerbose(name, args)
			},
			OnToolResult: func(name, result string, isError bool) {
				e.printer.ToolResult(name, result, isError)
			},
			OnResponse: func(text string) {
				e.printer.AgentResponse(text)
			},
		}
	}

	a := agent.NewAgent(provider,
		agent.WithTools(resolvedTools...),
		agent.WithModel(agentCfg.Model),
		agent.WithSystemPrompt(systemPrompt),
		agent.WithMaxTurns(agentCfg.MaxIterations),
		agent.WithTemperature(agentCfg.Temperature),
		agent.WithLogger(slog.Default()),
		agent.WithEventHandler(events),
	)

	agentOutput, runErr := a.Run(ctx, "Begin your task.")
	if runErr != nil {
		return nil, runErr
	}

	return &StepOutput{
		StepName: step.Name,
		Data:     agentOutput,
		Duration: time.Since(start),
	}, nil
}

func buildSystemPrompt(instructions string, triggerData *trigger.TriggerData, prevOutputs []StepOutput) string {
	hasTrigger := triggerData != nil && triggerData.Type != ""
	hasPrev := len(prevOutputs) > 0

	if !hasTrigger && !hasPrev {
		return instructions
	}

	var prompt string

	if hasTrigger {
		prompt += "## Trigger\n\n"
		prompt += "Type: " + triggerData.Type + "\n"
		prompt += "Source: " + triggerData.Source + "\n"
		if triggerData.Intent != "" {
			prompt += "Intent: " + triggerData.Intent + "\n"
		}
		if triggerData.URL != "" {
			prompt += "URL: " + triggerData.URL + "\n"
		}
		if triggerData.Author != "" {
			prompt += "Author: " + triggerData.Author + "\n"
		}
		prompt += "\n"
	}

	if hasPrev {
		prompt += "## Previous Steps\n\n"
		for _, prev := range prevOutputs {
			prompt += "### " + prev.StepName + "\n\n"
			prompt += prev.Data + "\n\n"
		}
	}

	prompt += "---\n\n"
	prompt += instructions
	return prompt
}
