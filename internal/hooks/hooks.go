// Package hooks provides lifecycle hook execution for Jerry workflows.
package hooks

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"time"
)

const hookTimeout = 10 * time.Second

// Event names for lifecycle hooks.
const (
	OnWorkflowStart    = "on_workflow_start"
	OnWorkflowComplete = "on_workflow_complete"
	OnWorkflowFailure  = "on_workflow_failure"
	OnStepStart        = "on_step_start"
	OnStepComplete     = "on_step_complete"
	OnStepFailure      = "on_step_failure"
	BeforeToolCall     = "before_tool_call"
	AfterToolCall      = "after_tool_call"
)

// ValidEvents lists all recognized hook event names.
var ValidEvents = []string{
	OnWorkflowStart, OnWorkflowComplete, OnWorkflowFailure,
	OnStepStart, OnStepComplete, OnStepFailure,
	BeforeToolCall, AfterToolCall,
}

// HookDef defines a single hook — a shell command with an optional tool filter.
type HookDef struct {
	Run   string   `yaml:"run"`
	Tools []string `yaml:"tools,omitempty"`
}

// Hooks maps event names to lists of hook definitions.
type Hooks map[string][]HookDef

// Runner executes hooks with environment context.
type Runner struct {
	hooks     Hooks
	repoRoot  string
	secretEnv []string
	baseEnv   map[string]string
}

// NewRunner creates a hook runner.
func NewRunner(hooks Hooks, repoRoot string, secretEnv []string) *Runner {
	return &Runner{
		hooks:     hooks,
		repoRoot:  repoRoot,
		secretEnv: secretEnv,
		baseEnv:   make(map[string]string),
	}
}

// SetBaseEnv sets environment variables included in every hook invocation.
func (r *Runner) SetBaseEnv(env map[string]string) {
	r.baseEnv = env
}

// Fire executes all hooks registered for the given event.
// Extra env vars are merged with the base env. For tool events, toolName
// is matched against each hook's Tools filter.
func (r *Runner) Fire(event string, env map[string]string) {
	if r == nil || r.hooks == nil {
		return
	}

	defs, ok := r.hooks[event]
	if !ok {
		return
	}

	toolName := env["JERRY_HOOK_TOOL_NAME"]

	for _, def := range defs {
		if len(def.Tools) > 0 && toolName != "" {
			if !containsTool(def.Tools, toolName) {
				continue
			}
		}

		r.exec(event, def.Run, env)
	}
}

func (r *Runner) exec(event, command string, extraEnv map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = r.repoRoot

	env := buildHookEnv(r.secretEnv, r.baseEnv, extraEnv)
	cmd.Env = env

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Warn("hook failed",
			"event", event,
			"command", truncate(command, 80),
			"error", err,
			"stderr", truncate(stderr.String(), 200),
		)
	}
}

func buildHookEnv(secretEnv []string, base, extra map[string]string) []string {
	env := make([]string, 0, 2+len(secretEnv)+len(base)+len(extra))
	env = append(env, "PATH="+os.Getenv("PATH"), "HOME="+os.Getenv("HOME"))
	env = append(env, secretEnv...)
	for k, v := range base {
		env = append(env, k+"="+v)
	}
	for k, v := range extra {
		env = append(env, k+"="+os.ExpandEnv(v))
	}
	return env
}

func containsTool(tools []string, name string) bool {
	return slices.Contains(tools, name)
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
