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
//
// The Executor creates a Loop per-execution rather than holding a shared Loop
// instance. This is intentional — the Loop holds no state between runs, and
// creating it fresh avoids shared-state concerns.
type Executor struct {
	loader   *Loader
	registry *tools.Registry
	client   llm.Client
	printer  *output.Printer
}

// NewExecutor creates an agent executor.
// client may be nil — in that case, Execute returns a clear error for agent steps.
func NewExecutor(loader *Loader, registry *tools.Registry, client llm.Client, printer *output.Printer) *Executor {
	return &Executor{
		loader:   loader,
		registry: registry,
		client:   client,
		printer:  printer,
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
			"agent step requires ANTHROPIC_API_KEY to be set")
	}

	start := time.Now()

	// Load agent config.
	config, loadErr := e.loader.Load(step.Agent)
	if loadErr != nil {
		return nil, loadErr
	}

	// Resolve tools: convert agent.ToolAccess → tools.ToolAccess.
	toolAccess := make([]tools.ToolAccess, len(config.Tools))
	for i, ta := range config.Tools {
		toolAccess[i] = tools.ToolAccess{
			Name:        ta.Name,
			Constraints: ta.Constraints,
		}
	}

	toolDefs, dispatch, resolveErr := e.registry.Resolve(toolAccess)
	if resolveErr != nil {
		return nil, motifErrors.Wrap(motifErrors.CodeToolNotFound,
			fmt.Sprintf("agent %q: tool resolution failed", config.Name), resolveErr)
	}

	// Build pipeline context scoped by context_access.
	// GetKeys only reads from Data map, so we inject trigger explicitly.
	contextData := store.GetKeys(config.ContextAccess)

	// Trigger is always available to agents regardless of context_access.
	fullContext := store.Get()
	contextData["trigger"] = fullContext.Trigger

	// Create and run the agentic loop.
	agentLoop := NewLoop(e.client, e.printer)
	loopResult, loopErr := agentLoop.Run(stepCtx, *config, toolDefs, dispatch, contextData)
	if loopErr != nil {
		return nil, loopErr
	}

	// Parse and validate output. On failure, retry once with a correction prompt.
	parsedOutput := parseAgentOutput(stepCtx, agentLoop, loopResult, config, toolDefs, dispatch, e.printer)

	return &pipeline.StepOutput{
		Data:              parsedOutput,
		Stdout:            loopResult.RawOutput,
		Duration:          time.Since(start),
		OutputKeyOverride: config.OutputKey,
	}, nil
}

// parseAgentOutput parses the agent's raw output as JSON, retrying once with a
// correction prompt if the initial parse fails. Returns nil (not an error) if
// both attempts fail — the pipeline continues with raw text in Stdout.
func parseAgentOutput(
	stepCtx context.Context,
	agentLoop *Loop,
	loopResult *LoopResult,
	config *AgentConfig,
	toolDefs []llm.ToolDef,
	dispatch func(context.Context, llm.ToolCall) (string, error),
	printer *output.Printer,
) map[string]any {
	parsed, parseErr := ParseOutput(loopResult.RawOutput, config.OutputSchema)
	if parseErr == nil {
		return parsed
	}

	// Retry once with a correction prompt.
	correctionPrompt := fmt.Sprintf(
		"Your response was not valid JSON matching the required schema. "+
			"Error: %s\n\nPlease respond with ONLY a JSON object matching the schema. "+
			"No markdown fences, no explanation — raw JSON only.", parseErr)

	retryResult, retryErr := agentLoop.RunRetry(stepCtx, loopResult, toolDefs, dispatch, correctionPrompt)
	if retryErr != nil {
		printer.Warning("agent %q: output retry failed: %s", config.Name, retryErr)
		return nil
	}

	parsed, parseErr = ParseOutput(retryResult.RawOutput, config.OutputSchema)
	if parseErr != nil {
		printer.Warning("agent %q: output is not valid JSON after retry, storing raw text", config.Name)
		return nil
	}

	return parsed
}
