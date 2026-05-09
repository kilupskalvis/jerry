package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kilupskalvis/jerry/internal/agent"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/tool"
)

var _ StepExecutor = (*AgentExecutor)(nil)

// AgentExecutor runs agent steps in the workflow.
type AgentExecutor struct {
	loader       *agent.Loader
	registry     *tool.Registry
	anthropicKey string
	openaiKey    string

	ProviderOverride llm.Provider
}

func NewAgentExecutor(loader *agent.Loader, registry *tool.Registry, anthropicKey, openaiKey string) *AgentExecutor {
	return &AgentExecutor{
		loader:       loader,
		registry:     registry,
		anthropicKey: anthropicKey,
		openaiKey:    openaiKey,
	}
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
		provider, provErr = llm.NewProviderForModel(agentCfg.Model, agentCfg.Provider, e.anthropicKey, e.openaiKey)
		if provErr != nil {
			return nil, jerrerr.Wrap(jerrerr.CodeLLMAuthFailed,
				fmt.Sprintf("agent %q", agentCfg.Name), provErr)
		}
	}

	resolvedTools, resolveErr := e.registry.Resolve(agentCfg.Tools)
	if resolveErr != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeToolNotFound,
			fmt.Sprintf("agent %q", agentCfg.Name), resolveErr)
	}

	systemPrompt := buildSystemPrompt(agentCfg.Instructions, prevOutputs)

	a := agent.NewAgent(provider,
		agent.WithTools(resolvedTools...),
		agent.WithModel(agentCfg.Model),
		agent.WithSystemPrompt(systemPrompt),
		agent.WithMaxTurns(agentCfg.MaxIterations),
		agent.WithLogger(slog.Default()),
	)

	output, runErr := a.Run(ctx, "Begin your task.")
	if runErr != nil {
		return nil, runErr
	}

	return &StepOutput{
		StepName: step.Name,
		Data:     output,
		Duration: time.Since(start),
	}, nil
}

func buildSystemPrompt(instructions string, prevOutputs []StepOutput) string {
	if len(prevOutputs) == 0 {
		return instructions
	}

	var prompt string
	prompt += "## Previous Steps\n\n"
	for _, prev := range prevOutputs {
		prompt += "### " + prev.StepName + "\n\n"
		prompt += prev.Data + "\n\n"
	}
	prompt += "---\n\n"
	prompt += instructions
	return prompt
}
