package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kilupskalvis/jerry/internal/agent"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/hooks"
	"github.com/kilupskalvis/jerry/internal/llm"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/permissions"
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

	settingsPerms permissions.Permissions
	hookRunner    *hooks.Runner

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

// SetPermissions sets project-level permissions loaded from settings files.
func (e *AgentExecutor) SetPermissions(perms permissions.Permissions) {
	e.settingsPerms = perms
}

// SetHookRunner sets the hook runner for tool-level lifecycle hooks.
func (e *AgentExecutor) SetHookRunner(r *hooks.Runner) {
	e.hookRunner = r
}

// Registry returns the tool registry for external agent-tool loading.
func (e *AgentExecutor) Registry() *tool.Registry {
	return e.registry
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

	e.loadAgentTools(step.Agent, provider)

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
			OnTurn: func(turn int, stopReason string, toolCalls, inputTokens, outputTokens, cacheCreation, cacheRead int) {
				e.printer.AgentTurn(turn, stopReason, toolCalls, inputTokens, outputTokens, cacheCreation, cacheRead)
			},
			OnToolCall: func(name, args string) {
				e.printer.ToolCallVerbose(name, args)
				if e.hookRunner != nil {
					e.hookRunner.Fire(hooks.BeforeToolCall, map[string]string{
						"JERRY_HOOK_STEP_NAME":  step.Name,
						"JERRY_HOOK_TOOL_NAME":  name,
						"JERRY_HOOK_TOOL_INPUT": args,
					})
				}
			},
			OnToolResult: func(name, result string, isError bool) {
				e.printer.ToolResult(name, result, isError)
				if e.hookRunner != nil {
					e.hookRunner.Fire(hooks.AfterToolCall, map[string]string{
						"JERRY_HOOK_STEP_NAME":     step.Name,
						"JERRY_HOOK_TOOL_NAME":     name,
						"JERRY_HOOK_TOOL_OUTPUT":   result,
						"JERRY_HOOK_TOOL_IS_ERROR": fmt.Sprintf("%v", isError),
					})
				}
			},
			OnResponse: func(text string) {
				e.printer.AgentResponse(text)
			},
		}
	}

	mergedPerms := e.settingsPerms.Merge(agentCfg.Permissions)
	var checker permissions.Checker
	if len(mergedPerms.Deny) > 0 || len(mergedPerms.Allow) > 0 {
		checker = permissions.NewChecker(mergedPerms, "merged")
	}

	a := agent.NewAgent(provider,
		agent.WithTools(resolvedTools...),
		agent.WithModel(agentCfg.Model),
		agent.WithSystemPrompt(systemPrompt),
		agent.WithMaxTurns(agentCfg.MaxIterations),
		agent.WithTemperature(agentCfg.Temperature),
		agent.WithLogger(slog.Default()),
		agent.WithEventHandler(events),
		agent.WithChecker(checker),
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

// loadAgentTools discovers sibling .md files in the workflow directory and registers
// them as agent-tools. Subagents get built-in + CI + custom tools only (no recursion).
func (e *AgentExecutor) loadAgentTools(agentPath string, parentProvider llm.Provider) {
	e.registry.ClearAgentTools()
	workflowDir := filepath.Dir(agentPath)

	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(workflowDir, entry.Name())
		subCfg, loadErr := e.loader.Load(path)
		if loadErr != nil {
			continue
		}

		subProvider := parentProvider
		if e.ProviderOverride == nil && subCfg.Model != "" {
			if resolved, err := e.resolver.ForModel(subCfg.Model, subCfg.Provider); err == nil {
				subProvider = resolved
			}
		}

		var triggerData *trigger.TriggerData
		if e.store != nil {
			t := e.store.Trigger()
			triggerData = &t
		}

		subTools := e.registry.BaseTools()
		subOptIn, _ := e.registry.Resolve(subCfg.Tools)
		subTools = append(subTools, subOptIn...)

		mergedPerms := e.settingsPerms.Merge(subCfg.Permissions)
		finalProvider := subProvider

		subName := subCfg.Name
		runFunc := func(ctx context.Context, task string) (string, error) {
			start := time.Now()
			if e.printer != nil {
				e.printer.SubagentStart(subName)
			}

			systemPrompt := subCfg.Instructions
			if triggerData != nil && triggerData.Type != "" {
				systemPrompt = buildTriggerPrefix(triggerData) + systemPrompt
			}

			var checker permissions.Checker
			if len(mergedPerms.Deny) > 0 || len(mergedPerms.Allow) > 0 {
				checker = permissions.NewChecker(mergedPerms, "subagent:"+subName)
			}

			var events *agent.EventHandler
			if e.printer != nil {
				events = &agent.EventHandler{
					OnTurn: func(turn int, stopReason string, toolCalls, inputTokens, outputTokens, cacheCreation, cacheRead int) {
						e.printer.SubagentTurn(turn, stopReason, toolCalls, inputTokens, outputTokens, cacheCreation, cacheRead)
					},
					OnToolCall: func(name, args string) {
						e.printer.SubagentToolCallVerbose(name, args)
					},
					OnToolResult: func(name, result string, isError bool) {
						e.printer.SubagentToolResult(name, result, isError)
					},
				}
			}

			a := agent.NewAgent(finalProvider,
				agent.WithTools(subTools...),
				agent.WithModel(subCfg.Model),
				agent.WithSystemPrompt(systemPrompt),
				agent.WithMaxTurns(subCfg.MaxIterations),
				agent.WithTemperature(subCfg.Temperature),
				agent.WithLogger(slog.Default()),
				agent.WithChecker(checker),
				agent.WithEventHandler(events),
			)

			output, err := a.Run(ctx, task)
			if e.printer != nil {
				e.printer.SubagentSuccess(subName, time.Since(start))
			}
			return output, err
		}

		e.registry.RegisterAgentTool(
			tool.NewAgentTool(subCfg.Name, subCfg.Instructions, runFunc),
		)
	}
}

func formatTriggerSection(td *trigger.TriggerData) string {
	var b strings.Builder
	b.WriteString("## Trigger\n\n")
	b.WriteString("Type: " + td.Type + "\n")
	b.WriteString("Source: " + td.Source + "\n")
	if td.Intent != "" {
		b.WriteString("Intent: " + td.Intent + "\n")
	}
	if td.URL != "" {
		b.WriteString("URL: " + td.URL + "\n")
	}
	if td.Author != "" {
		b.WriteString("Author: " + td.Author + "\n")
	}
	for key, value := range td.Metadata {
		if value == "" {
			continue
		}
		label := strings.ReplaceAll(key, "_", " ")
		label = strings.ToUpper(label[:1]) + label[1:]
		b.WriteString(label + ": " + value + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

func buildTriggerPrefix(td *trigger.TriggerData) string {
	return formatTriggerSection(td) + "---\n\n"
}

func buildSystemPrompt(instructions string, triggerData *trigger.TriggerData, prevOutputs []StepOutput) string {
	hasTrigger := triggerData != nil && triggerData.Type != ""
	hasPrev := len(prevOutputs) > 0

	if !hasTrigger && !hasPrev {
		return instructions
	}

	var prompt string

	if hasTrigger {
		prompt += formatTriggerSection(triggerData)
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
