// Package agent implements the StepExecutor for AI agent steps.
// It loads agent definitions from markdown files, runs the agentic loop
// (think → tool call → observe → repeat), and returns structured output.
package agent

import (
	"context"
	"fmt"
	"time"

	motifErrors "github.com/kilupskalvis/motif/internal/errors"
	"github.com/kilupskalvis/motif/internal/llm"
	"github.com/kilupskalvis/motif/internal/output"
	"github.com/kilupskalvis/motif/internal/pipeline"
	"github.com/kilupskalvis/motif/internal/tools"
)

// Compile-time interface compliance assertion.
var _ pipeline.StepExecutor = (*Executor)(nil)

// Executor runs agent steps in the pipeline. It loads the agent definition,
// resolves tools, runs the agentic loop, and parses the output.
type Executor struct {
	loader    *Loader
	registry  *tools.Registry
	client    llm.Client
	compactor *llm.Compactor
	printer   *output.Printer
}

// NewExecutor creates an agent executor.
// client and compactor may be nil — Execute returns a clear error for agent steps
// if no client is available.
func NewExecutor(loader *Loader, registry *tools.Registry, client llm.Client, compactor *llm.Compactor, printer *output.Printer) *Executor {
	return &Executor{
		loader:    loader,
		registry:  registry,
		client:    client,
		compactor: compactor,
		printer:   printer,
	}
}

// CanExecute returns true if the step has an Agent field set.
func (e *Executor) CanExecute(step pipeline.Step) bool {
	return step.Agent != ""
}

// Execute loads the agent definition, resolves tools, runs the agentic loop,
// parses the output, and returns a StepOutput with OutputKeyOverride set to
// the agent's output_key.
func (e *Executor) Execute(stepCtx context.Context, step pipeline.Step, store pipeline.ContextReader) (*pipeline.StepOutput, error) {
	if e.client == nil {
		return nil, motifErrors.New(motifErrors.CodeLLMAuthFailed,
			"agent step requires an LLM API key (ANTHROPIC_API_KEY or OPENAI_API_KEY)")
	}

	start := time.Now()

	agentCfg, loadErr := e.loader.Load(step.Agent)
	if loadErr != nil {
		return nil, loadErr
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
		return nil, motifErrors.Wrap(motifErrors.CodeToolNotFound,
			fmt.Sprintf("agent %q: tool resolution failed", agentCfg.Name), resolveErr)
	}

	// Build pipeline context scoped by context_access.
	contextData := store.GetKeys(agentCfg.ContextAccess)

	// Trigger is always available to agents regardless of context_access.
	fullContext := store.Get()
	contextData["trigger"] = fullContext.Trigger

	agentLoop := NewLoop(e.client, e.compactor, e.printer)
	loopResult, loopErr := agentLoop.Run(stepCtx, *agentCfg, toolDefs, dispatch, contextData)
	if loopErr != nil {
		return nil, loopErr
	}

	parsedOutput := parseAgentOutput(stepCtx, agentLoop, loopResult, agentCfg, toolDefs, dispatch, e.printer)

	return &pipeline.StepOutput{
		Data:              parsedOutput,
		Stdout:            loopResult.RawOutput,
		Duration:          time.Since(start),
		OutputKeyOverride: agentCfg.OutputKey,
	}, nil
}

// parseAgentOutput parses the agent's raw output as JSON, retrying once with a
// correction prompt if the initial parse fails. Returns nil (not an error) if
// both attempts fail — the pipeline continues with raw text in Stdout.
func parseAgentOutput(
	stepCtx context.Context,
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

	retryResult, retryErr := agentLoop.RunRetry(stepCtx, loopResult, toolDefs, dispatch, correctionPrompt)
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
