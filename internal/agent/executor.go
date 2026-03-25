// Package agent implements the StepExecutor for AI agent steps.
package agent

import (
	"context"
	"fmt"
	"time"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/pipeline"
	"github.com/kilupskalvis/jerry/internal/tools"
)

// Compile-time interface compliance assertion.
var _ pipeline.StepExecutor = (*Executor)(nil)

// Executor runs agent steps in the pipeline. It loads the agent definition,
// resolves the LLM client per-agent (based on model/provider), runs the
// agentic loop, and parses the output.
type Executor struct {
	loader       *Loader
	registry     *tools.Registry
	anthropicKey string
	openaiKey    string
	printer      *output.Printer

	// ClientOverride, if set, is used instead of resolving a client per-agent.
	// Used in tests to inject a mock LLM client.
	ClientOverride llm.Client
}

// NewExecutor creates an agent executor with the given API keys.
// The LLM client is resolved per-execution based on each agent's model field.
func NewExecutor(loader *Loader, registry *tools.Registry, anthropicKey, openaiKey string, printer *output.Printer) *Executor {
	return &Executor{
		loader:       loader,
		registry:     registry,
		anthropicKey: anthropicKey,
		openaiKey:    openaiKey,
		printer:      printer,
	}
}

// CanExecute returns true if the step has an Agent field set.
func (e *Executor) CanExecute(step pipeline.Step) bool {
	return step.Agent != ""
}

// Execute loads the agent definition, resolves the LLM client and tools,
// runs the agentic loop, parses the output, and returns a StepOutput.
func (e *Executor) Execute(ctx context.Context, step pipeline.Step, store pipeline.ContextReader) (*pipeline.StepOutput, error) {
	start := time.Now()

	agentCfg, loadErr := e.loader.Load(step.Agent)
	if loadErr != nil {
		return nil, loadErr
	}

	// Resolve LLM client for this agent's model.
	client := e.ClientOverride
	if client == nil {
		var clientErr error
		client, clientErr = llm.NewClientForModel(agentCfg.Model, agentCfg.Provider, e.anthropicKey, e.openaiKey)
		if clientErr != nil {
			return nil, jerrerr.Wrap(jerrerr.CodeLLMAuthFailed,
				fmt.Sprintf("agent %q", agentCfg.Name), clientErr)
		}
	}

	// Resolve tools: convert agent.ToolAccess → tools.ToolAccess.
	toolAccess := make([]tools.ToolAccess, len(agentCfg.Tools))
	for i, ta := range agentCfg.Tools {
		toolAccess[i] = tools.ToolAccess{
			Name:        ta.Name,
			Constraints: ta.Constraints,
		}
	}

	toolDefs, dispatch, resolveErr := e.registry.Resolve(toolAccess)
	if resolveErr != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeToolNotFound,
			fmt.Sprintf("agent %q: tool resolution failed", agentCfg.Name), resolveErr)
	}

	// Build pipeline context scoped by context_access.
	contextData := store.GetKeys(agentCfg.ContextAccess)

	// Trigger is always available to agents regardless of context_access.
	fullContext := store.Get()
	contextData["trigger"] = fullContext.Trigger

	compactor := llm.NewCompactor(client)
	agentLoop := NewLoop(client, compactor, e.printer)
	loopResult, loopErr := agentLoop.Run(ctx, *agentCfg, toolDefs, dispatch, contextData)
	if loopErr != nil {
		return nil, loopErr
	}

	parsedOutput := parseAgentOutput(ctx, agentLoop, loopResult, agentCfg, toolDefs, dispatch, e.printer)

	return &pipeline.StepOutput{
		Data:              parsedOutput,
		Stdout:            loopResult.RawOutput,
		Duration:          time.Since(start),
		OutputKeyOverride: agentCfg.OutputKey,
		Iterations:        loopResult.Iterations,
		ToolCalls:         loopResult.ToolCalls,
		TokensInput:       loopResult.TotalUsage.InputTokens,
		TokensOutput:      loopResult.TotalUsage.OutputTokens,
	}, nil
}

// parseAgentOutput parses the agent's raw output as JSON, retrying once with a
// correction prompt if the initial parse fails. Returns nil (not an error) if
// both attempts fail — the pipeline continues with raw text in Stdout.
func parseAgentOutput(
	ctx context.Context,
	agentLoop *Loop,
	loopResult *LoopResult,
	agentCfg *AgentConfig,
	toolDefs []llm.ToolDef,
	dispatch func(context.Context, llm.ToolCall) (string, error),
	printer *output.Printer,
) map[string]any {
	parsed, parseErr := ParseOutput(loopResult.RawOutput, agentCfg.OutputSchema)
	if parseErr == nil {
		return parsed
	}

	correctionPrompt := fmt.Sprintf(
		"Your response was not valid JSON matching the required schema. "+
			"Error: %s\n\nPlease respond with ONLY a JSON object matching the schema. "+
			"No markdown fences, no explanation — raw JSON only.", parseErr)

	retryResult, retryErr := agentLoop.RunRetry(ctx, loopResult, toolDefs, dispatch, correctionPrompt)
	if retryErr != nil {
		printer.Warning("agent %q: output retry failed: %s", agentCfg.Name, retryErr)
		return nil
	}

	parsed, parseErr = ParseOutput(retryResult.RawOutput, agentCfg.OutputSchema)
	if parseErr != nil {
		printer.Warning("agent %q: output is not valid JSON after retry, storing raw text", agentCfg.Name)
		return nil
	}

	return parsed
}
