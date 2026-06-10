// Package spec defines the v3 Jerry spec model: the .jerry/ directory
// contents (workflow.yaml, prompt files, settings.yaml, jerry.lock), strict
// loading, and authoring-time validation.
package spec

// Workflow is one parsed workflow.yaml.
type Workflow struct {
	Version  int      `yaml:"version"`
	On       Triggers `yaml:"on"`
	Defaults Defaults `yaml:"defaults,omitempty"`
	Env      []string `yaml:"env,omitempty"`
	Steps    []Step   `yaml:"steps"`

	// Set by the loader, not from YAML.
	Name string `yaml:"-"`
	Dir  string `yaml:"-"`
}

// Triggers mirrors the spec `on:` block. At least one must be set.
type Triggers struct {
	PullRequest *PullRequestTrigger `yaml:"pull_request,omitempty"`
	Push        *PushTrigger        `yaml:"push,omitempty"`
	Dispatch    *DispatchTrigger    `yaml:"dispatch,omitempty"`
	Schedule    *ScheduleTrigger    `yaml:"schedule,omitempty"`
}

// None reports whether no trigger is configured.
func (t Triggers) None() bool {
	return t.PullRequest == nil && t.Push == nil && t.Dispatch == nil && t.Schedule == nil
}

type PullRequestTrigger struct {
	Types []string `yaml:"types,omitempty"`
}

type PushTrigger struct {
	Branches []string `yaml:"branches,omitempty"`
}

type DispatchTrigger struct {
	Types []string `yaml:"types,omitempty"`
}

type ScheduleTrigger struct {
	Cron string `yaml:"cron"`
}

// Defaults applies to every agent step unless the step overrides.
type Defaults struct {
	Runtime string `yaml:"runtime,omitempty"`
	Model   string `yaml:"model,omitempty"`
}

// DefaultRuntime is used when neither the step nor defaults set one.
const DefaultRuntime = "pi"

// Step is one pipeline step. Exactly one of Prompt, Run, CI is set.
type Step struct {
	Name string `yaml:"name"`

	Prompt string `yaml:"prompt,omitempty"`
	Run    string `yaml:"run,omitempty"`
	CI     string `yaml:"ci,omitempty"`

	Runtime     string            `yaml:"runtime,omitempty"`
	Model       string            `yaml:"model,omitempty"`
	Context     []string          `yaml:"context,omitempty"`
	Outputs     map[string]string `yaml:"outputs,omitempty"`
	Permissions PermissionSet     `yaml:"permissions,omitempty"`
	Budget      Budget            `yaml:"budget,omitempty"`
	Timeout     Duration          `yaml:"timeout,omitempty"`
	Retries     int               `yaml:"retries,omitempty"`

	// Env narrows the workflow env for this step. nil = inherit all,
	// empty non-nil = no secrets (JERRY_* context vars only).
	Env *[]string `yaml:"env,omitempty"`

	// CI-action fields, valid only when CI is set.
	Body   string `yaml:"body,omitempty"`
	Status string `yaml:"status,omitempty"`
	Title  string `yaml:"title,omitempty"`
}

// EffectiveRuntime resolves the runtime for an agent step.
func (s *Step) EffectiveRuntime(d Defaults) string {
	if s.Runtime != "" {
		return s.Runtime
	}
	if d.Runtime != "" {
		return d.Runtime
	}
	return DefaultRuntime
}

// PermissionSet is the policy a step grants its runtime. Patterns use the
// noun(selector) grammar: "read", "bash(go test:*)", "write(.env)".
// Deny always wins over allow.
type PermissionSet struct {
	Allow []string `yaml:"allow,omitempty"`
	Deny  []string `yaml:"deny,omitempty"`
}

// Budget caps a step's spend. Zero values mean "no cap".
type Budget struct {
	MaxCost   float64 `yaml:"max_cost,omitempty"`
	MaxTokens int64   `yaml:"max_tokens,omitempty"`
}
