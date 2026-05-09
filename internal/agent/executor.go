// Package agent implements the StepExecutor for AI agent steps.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/tools"
	"github.com/kilupskalvis/jerry/internal/workflow"
)

var _ workflow.StepExecutor = (*Executor)(nil)

// Executor runs agent steps in the workflow.
type Executor struct {
	loader       *Loader
	registry     *tools.Registry
	anthropicKey string
	openaiKey    string

	ProviderOverride Provider
}

func NewExecutor(loader *Loader, registry *tools.Registry, anthropicKey, openaiKey string) *Executor {
	return &Executor{
		loader:       loader,
		registry:     registry,
		anthropicKey: anthropicKey,
		openaiKey:    openaiKey,
	}
}

func (e *Executor) CanExecute(step workflow.Step) bool {
	return step.Agent != ""
}

func (e *Executor) Execute(ctx context.Context, step workflow.Step, prevOutputs []workflow.StepOutput) (*workflow.StepOutput, error) {
	start := time.Now()

	agentCfg, err := e.loader.Load(step.Agent)
	if err != nil {
		return nil, err
	}

	provider := e.ProviderOverride
	if provider == nil {
		var provErr error
		provider, provErr = NewProviderForModel(agentCfg.Model, agentCfg.Provider, e.anthropicKey, e.openaiKey)
		if provErr != nil {
			return nil, jerrerr.Wrap(jerrerr.CodeLLMAuthFailed,
				fmt.Sprintf("agent %q", agentCfg.Name), provErr)
		}
	}

	toolAccess := make([]tools.ToolAccess, len(agentCfg.Tools))
	for i, ta := range agentCfg.Tools {
		toolAccess[i] = tools.ToolAccess{Name: ta.Name, Constraints: ta.Constraints}
	}

	resolvedTools, resolveErr := e.registry.Resolve(toolAccess)
	if resolveErr != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeToolNotFound,
			fmt.Sprintf("agent %q", agentCfg.Name), resolveErr)
	}

	systemPrompt := buildSystemPrompt(agentCfg.Instructions, prevOutputs)

	a := NewAgent(provider,
		WithTools(wrapRegistryTools(resolvedTools)...),
		WithModel(agentCfg.Model),
		WithSystemPrompt(systemPrompt),
		WithMaxTurns(agentCfg.MaxIterations),
		WithLogger(slog.Default()),
	)

	output, runErr := a.Run(ctx, "Begin your task.")
	if runErr != nil {
		return nil, runErr
	}

	return &workflow.StepOutput{
		StepName: step.Name,
		Data:     output,
		Duration: time.Since(start),
	}, nil
}

func buildSystemPrompt(instructions string, prevOutputs []workflow.StepOutput) string {
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

func wrapRegistryTools(registryTools []tools.Tool) []Tool {
	result := make([]Tool, len(registryTools))
	for i, t := range registryTools {
		result[i] = NewToolFunc(t.ToolName, t.ToolDescription, t.Schema, t.Execute)
	}
	return result
}
