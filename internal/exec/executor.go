// Package exec implements single-step execution: the only code that runs
// in CI. It owns no sequencing — what step runs next is the CI platform's
// (or the local loop's) decision.
package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kilupskalvis/jerry/internal/budget"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/handoff"
	"github.com/kilupskalvis/jerry/internal/runtime"
	"github.com/kilupskalvis/jerry/internal/spec"
	"github.com/kilupskalvis/jerry/internal/trigger"
	"github.com/kilupskalvis/jerry/internal/workspace"
)

// Exit codes per the master spec §5.6.
const (
	ExitOK      = 0
	ExitStep    = 1
	ExitConfig  = 2
	ExitRuntime = 3
	ExitBudget  = 4
)

// Options are the executor's injected dependencies.
type Options struct {
	RepoRoot string
	JerryDir string
	Registry *runtime.Registry
	Out      io.Writer
}

// Executor runs exactly one step per Run call. Stateless between calls.
type Executor struct {
	opts Options
}

// New constructs an Executor.
func New(opts Options) *Executor { return &Executor{opts: opts} }

// Request identifies one step to execute.
type Request struct {
	Workflow string
	Step     string
	CtxDir   string
	// Trigger, when non-nil, is recorded into the ctx dir if absent
	// (first step of a run). Later steps reuse the recorded one.
	Trigger *trigger.TriggerData
	CILive  bool
}

// Run executes the step and returns the process exit code.
func (e *Executor) Run(ctx context.Context, req Request) int {
	code, err := e.run(ctx, req)
	if err != nil {
		fmt.Fprintf(e.opts.Out, "jerry exec: %v\n", err)
	}
	return code
}

func (e *Executor) run(ctx context.Context, req Request) (int, error) {
	project, err := spec.LoadProject(e.opts.JerryDir)
	if err != nil {
		return ExitConfig, err
	}
	var wf *spec.Workflow
	for _, w := range project.Workflows {
		if w.Name == req.Workflow {
			wf = w
		}
	}
	if wf == nil {
		return ExitConfig, jerrerr.New(jerrerr.CodeWorkflowNotFound,
			fmt.Sprintf("workflow %q not found under %s", req.Workflow, e.opts.JerryDir))
	}
	if issues := spec.ValidateProject(project); spec.HasErrors(issues) {
		return ExitConfig, fmt.Errorf("spec validation failed; run `jerry validate`")
	}

	var step *spec.Step
	stepIdx := -1
	for i := range wf.Steps {
		if wf.Steps[i].Name == req.Step {
			step, stepIdx = &wf.Steps[i], i
		}
	}
	if step == nil {
		return ExitConfig, fmt.Errorf("step %q not found in workflow %q", req.Step, req.Workflow)
	}

	dir := handoff.NewCtxDir(req.CtxDir)
	td, err := dir.ReadTrigger()
	if err != nil {
		if req.Trigger == nil {
			return ExitConfig, fmt.Errorf("no trigger recorded and none provided: %w", err)
		}
		td = *req.Trigger
		if err := dir.WriteTrigger(td); err != nil {
			return ExitRuntime, err
		}
	}

	ledger, err := budget.Load(dir.LedgerFile())
	if err != nil {
		return ExitConfig, err
	}

	runCtx := e.buildRunContext(wf, stepIdx, dir, td, ledger)

	switch step.Kind() {
	case spec.KindAgent:
		return e.runAgentStep(ctx, project, wf, step, dir, runCtx, ledger)
	case spec.KindShell:
		return e.runShellStep(ctx, step, dir, runCtx)
	case spec.KindCI:
		return e.runCIStep(step, dir, runCtx, req.CILive)
	}
	return ExitConfig, fmt.Errorf("step %q has no valid kind", step.Name)
}

// buildRunContext loads every prior step's record (execution order) plus
// run totals into template state.
func (e *Executor) buildRunContext(wf *spec.Workflow, stepIdx int, dir *handoff.CtxDir,
	td trigger.TriggerData, ledger *budget.Ledger) *handoff.RunContext {

	rc := &handoff.RunContext{
		Trigger: td,
		Steps:   map[string]handoff.StepRecord{},
	}
	cost, tokens := ledger.Totals()
	rc.Run = handoff.RunMeta{ID: filepath.Base(dir.Root()), Cost: cost, Tokens: tokens}

	for i := range stepIdx {
		name := wf.Steps[i].Name
		rec, err := dir.ReadStep(name)
		if err != nil {
			continue // prior step skipped; refs to it fail in Resolve
		}
		rc.Steps[name] = rec
		rc.Order = append(rc.Order, name)
	}
	return rc
}

func (e *Executor) runAgentStep(ctx context.Context, project *spec.Project, wf *spec.Workflow,
	step *spec.Step, dir *handoff.CtxDir, runCtx *handoff.RunContext, ledger *budget.Ledger) (int, error) {

	adapter, err := e.opts.Registry.Lookup(step.EffectiveRuntime(wf.Defaults))
	if err != nil {
		return ExitConfig, err
	}

	instructions, err := wf.PromptText(step)
	if err != nil {
		return ExitConfig, err
	}
	prompt, err := handoff.BuildPrompt(instructions, step.Context, runCtx)
	if err != nil {
		return ExitConfig, err
	}

	perms := step.Permissions
	if project.Settings != nil {
		perms = perms.MergeDeny(project.Settings.Policy.Deny)
	}

	model := step.Model
	if model == "" {
		model = wf.Defaults.Model
	}

	pre, err := workspace.RecordState(e.opts.RepoRoot)
	if err != nil {
		return ExitRuntime, err
	}

	// Runtime timeout is enforced here so exec exits cleanly (code 3) with
	// a useful error before any outer CI timeout hard-kills the step.
	if step.Timeout.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, step.Timeout.Duration)
		defer cancel()
	}

	fmt.Fprintf(e.opts.Out, "▸ %s (%s)\n", step.Name, adapter.Name())
	result, err := adapter.Invoke(ctx, runtime.InvocationSpec{
		Prompt:       prompt,
		Workdir:      e.opts.RepoRoot,
		Model:        model,
		Permissions:  perms,
		OutputSchema: step.Outputs,
		Env:          envAllowlist(wf, step),
		Timeout:      step.Timeout.Duration,
	})

	// Usage is recorded even when the invocation failed: spent is spent.
	if result.Usage != nil {
		ledger.Record(step.Name, *result.Usage)
		if saveErr := ledger.Save(); saveErr != nil {
			return ExitRuntime, saveErr
		}
	}
	if err != nil {
		return ExitRuntime, fmt.Errorf("runtime %s failed: %w", adapter.Name(), err)
	}

	if err := validateOutputs(step, result.Outputs); err != nil {
		return ExitStep, err
	}

	snap, err := workspace.Capture(e.opts.RepoRoot, pre)
	if err != nil {
		return ExitRuntime, err
	}

	var usageJSON json.RawMessage
	if result.Usage != nil {
		usageJSON, _ = json.Marshal(result.Usage)
	}
	if err := dir.WriteStep(handoff.StepRecord{
		Name: step.Name, Output: result.Text, Outputs: result.Outputs,
		Diff: snap.Patch, DiffStat: snap.Stat, Usage: usageJSON,
	}); err != nil {
		return ExitRuntime, err
	}

	if err := ledger.CheckStep(step.Name, step.Budget); err != nil {
		return ExitBudget, err
	}
	var ceiling float64
	if project.Settings != nil {
		ceiling = project.Settings.Policy.Budget.MaxCostPerRun
	}
	if err := ledger.CheckRun(ceiling); err != nil {
		return ExitBudget, err
	}

	fmt.Fprintf(e.opts.Out, "✓ %s\n", step.Name)
	return ExitOK, nil
}

// envAllowlist resolves the step's effective env (workflow env is the
// default, step env narrows) into KEY=VALUE pairs from the parent process.
// Secrets the runtime needs flow only through here — never os.Environ().
func envAllowlist(wf *spec.Workflow, step *spec.Step) []string {
	names := wf.Env
	if step.Env != nil {
		names = *step.Env
	}
	env := make([]string, 0, len(names))
	for _, name := range names {
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return env
}

// validateOutputs enforces the declared schema: missing keys or wrong
// types fail the step; extra undeclared keys are ignored.
func validateOutputs(step *spec.Step, got map[string]any) error {
	for key, typ := range step.Outputs {
		v, ok := got[key]
		if !ok {
			return fmt.Errorf("step %q: runtime did not produce declared output %q", step.Name, key)
		}
		if !typeMatches(typ, v) {
			return fmt.Errorf("step %q: output %q is not a %s (got %T)", step.Name, key, typ, v)
		}
	}
	return nil
}

func typeMatches(typ string, v any) bool {
	switch typ {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "list":
		_, ok := v.([]any)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	}
	return false
}
