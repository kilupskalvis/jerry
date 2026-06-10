package spec

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/kilupskalvis/jerry/internal/handoff"
)

// Level is the severity of a validation issue.
type Level int

const (
	LevelWarning Level = iota
	LevelError
)

// Issue is one validation finding, printable as-is.
type Issue struct {
	Level    Level
	Workflow string
	Step     string
	Message  string
}

// HasErrors reports whether any issue is an error.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Level == LevelError {
			return true
		}
	}
	return false
}

var (
	stepNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

	ciActions = []string{
		"post_pr_comment", "post_review_comment",
		"add_check_status", "create_pull_request",
	}

	outputTypes = map[string]bool{
		"string": true, "number": true, "boolean": true,
		"list": true, "object": true,
	}
)

// SupportedVersion is the only spec version this binary understands.
const SupportedVersion = 1

// ValidateWorkflow checks one workflow in isolation. Cross-cutting policy
// (settings, lockfile) is ValidateProject's job.
func ValidateWorkflow(wf *Workflow) []Issue {
	var issues []Issue
	errf := func(step, format string, args ...any) {
		issues = append(issues, Issue{LevelError, wf.Name, step, fmt.Sprintf(format, args...)})
	}

	if wf.Version != SupportedVersion {
		errf("", "unsupported spec version %d (this jerry understands version %d)",
			wf.Version, SupportedVersion)
	}
	if wf.On.None() {
		errf("", "at least one trigger is required in `on:`")
	}
	if wf.On.Schedule != nil && wf.On.Schedule.Cron == "" {
		errf("", "schedule trigger requires `cron:`")
	}
	if len(wf.Steps) == 0 {
		errf("", "at least one step is required")
	}

	seen := map[string]bool{}
	for i := range wf.Steps {
		s := &wf.Steps[i]
		label := s.Name
		if s.Name == "" {
			errf("", "step %d: name is required", i+1)
			label = fmt.Sprintf("step %d", i+1)
		} else if !stepNameRe.MatchString(s.Name) {
			errf("", "invalid step name %q: must be kebab-case ([a-z0-9-])", s.Name)
		} else if seen[s.Name] {
			errf("", "duplicate step name %q", s.Name)
		}
		seen[s.Name] = true

		issues = append(issues, validateStep(wf, s, label)...)
	}
	issues = append(issues, validateRefs(wf)...)
	return issues
}

// validateRefs statically checks every ${{ }} reference and context: entry:
// referenced steps exist and run earlier; outputs keys are declared.
func validateRefs(wf *Workflow) []Issue {
	var issues []Issue

	stepIdx := map[string]int{}
	for i := range wf.Steps {
		stepIdx[wf.Steps[i].Name] = i
	}
	declared := func(name, key string) bool {
		i, ok := stepIdx[name]
		if !ok {
			return false
		}
		_, has := wf.Steps[i].Outputs[key]
		return has
	}

	check := func(pos int, where, text string) {
		errf := func(format string, args ...any) {
			issues = append(issues, Issue{LevelError, wf.Name, wf.Steps[pos].Name,
				fmt.Sprintf("step %q (%s): ", wf.Steps[pos].Name, where) +
					fmt.Sprintf(format, args...)})
		}

		refs, err := handoff.ExtractRefs(text)
		if err != nil {
			errf("%v", err)
			return
		}
		for _, r := range refs {
			switch r.Kind {
			case handoff.RefStepOutput, handoff.RefStepOutputs,
				handoff.RefStepDiff, handoff.RefStepDiffStat:
				target, ok := stepIdx[r.Step]
				if !ok {
					msg := fmt.Sprintf("unknown step %q", r.Step)
					if sug := Suggest(r.Step, stepNames(wf)); sug != "" {
						msg += fmt.Sprintf(" — did you mean %q?", sug)
					}
					errf("%s", msg)
					continue
				}
				if target >= pos {
					errf("step %q runs after %q — only earlier steps can be referenced",
						r.Step, wf.Steps[pos].Name)
					continue
				}
				if r.Kind == handoff.RefStepOutputs && !declared(r.Step, r.Key) {
					errf("step %q does not declare output %q", r.Step, r.Key)
				}
			}
		}
	}

	for i := range wf.Steps {
		s := &wf.Steps[i]

		switch s.Kind() {
		case KindAgent:
			text, err := wf.PromptText(s)
			if err != nil {
				issues = append(issues, Issue{LevelError, wf.Name, s.Name, err.Error()})
			} else {
				check(i, "prompt", text)
			}
		case KindShell:
			check(i, "run", s.Run)
		case KindCI:
			check(i, "body", s.Body)
			check(i, "status", s.Status)
			check(i, "title", s.Title)
		}

		for _, entry := range s.Context {
			target := ""
			switch {
			case entry == "trigger":
				continue
			case strings.HasPrefix(entry, "steps."):
				target = strings.TrimPrefix(entry, "steps.")
			case strings.HasPrefix(entry, "diff:"):
				target = strings.TrimPrefix(entry, "diff:")
			default:
				issues = append(issues, Issue{LevelError, wf.Name, s.Name,
					fmt.Sprintf("step %q: invalid context entry %q (trigger | steps.<name> | diff:<name>)",
						s.Name, entry)})
				continue
			}
			j, ok := stepIdx[target]
			if !ok || j >= i {
				issues = append(issues, Issue{LevelError, wf.Name, s.Name,
					fmt.Sprintf("step %q: context references unknown step %q (or one that runs later)",
						s.Name, target)})
			}
		}
	}
	return issues
}

// ValidateProject runs per-workflow validation plus cross-cutting policy
// checks: settings runtime allowlist, budget ceiling, lockfile coverage.
func ValidateProject(p *Project) []Issue {
	var issues []Issue
	for _, wf := range p.Workflows {
		issues = append(issues, ValidateWorkflow(wf)...)
		issues = append(issues, validatePolicy(p, wf)...)
	}
	return issues
}

func validatePolicy(p *Project, wf *Workflow) []Issue {
	var issues []Issue

	var ceiling float64
	var allowed []string
	if p.Settings != nil {
		ceiling = p.Settings.Policy.Budget.MaxCostPerRun
		allowed = p.Settings.Policy.Runtimes.Allowed
	}

	var totalDeclared float64
	for i := range wf.Steps {
		s := &wf.Steps[i]
		if s.Kind() != KindAgent {
			continue
		}
		rt := s.EffectiveRuntime(wf.Defaults)

		if len(allowed) > 0 && !slices.Contains(allowed, rt) {
			issues = append(issues, Issue{LevelError, wf.Name, s.Name,
				fmt.Sprintf("step %q: runtime %q is not allowed by settings.yaml (allowed: %v)",
					s.Name, rt, allowed)})
		}

		if p.Lock == nil {
			issues = append(issues, Issue{LevelWarning, wf.Name, s.Name,
				fmt.Sprintf("step %q: runtime %q is not pinned (no jerry.lock) — run `jerry lock`",
					s.Name, rt)})
		} else if entry, ok := p.Lock.Runtimes[rt]; !ok || entry.Version == "" {
			issues = append(issues, Issue{LevelError, wf.Name, s.Name,
				fmt.Sprintf("step %q: runtime %q is not pinned in jerry.lock", s.Name, rt)})
		}

		if s.Budget.MaxCost > 0 {
			totalDeclared += s.Budget.MaxCost
		} else if ceiling > 0 {
			issues = append(issues, Issue{LevelWarning, wf.Name, s.Name,
				fmt.Sprintf("step %q has no max_cost — run ceiling $%.2f cannot be statically guaranteed",
					s.Name, ceiling)})
		}
	}

	if ceiling > 0 && totalDeclared > ceiling {
		issues = append(issues, Issue{LevelError, wf.Name, "",
			fmt.Sprintf("declared step budgets total $%.2f, exceeding the $%.2f ceiling in settings.yaml",
				totalDeclared, ceiling)})
	}
	return issues
}

func stepNames(wf *Workflow) []string {
	names := make([]string, 0, len(wf.Steps))
	for i := range wf.Steps {
		if wf.Steps[i].Name != "" {
			names = append(names, wf.Steps[i].Name)
		}
	}
	return names
}

func validateStep(wf *Workflow, s *Step, label string) []Issue {
	var issues []Issue
	errf := func(format string, args ...any) {
		issues = append(issues, Issue{LevelError, wf.Name, label,
			fmt.Sprintf("step %q: ", label) + fmt.Sprintf(format, args...)})
	}

	kind := s.Kind()
	if kind == KindInvalid {
		errf("exactly one of prompt, run, ci must be set")
		return issues
	}

	if kind != KindAgent {
		agentOnly := []struct {
			field string
			set   bool
		}{
			{"outputs", len(s.Outputs) > 0},
			{"permissions", len(s.Permissions.Allow)+len(s.Permissions.Deny) > 0},
			{"context", len(s.Context) > 0},
			{"runtime", s.Runtime != ""},
			{"model", s.Model != ""},
			{"budget", s.Budget.MaxCost != 0 || s.Budget.MaxTokens != 0},
		}
		for _, f := range agentOnly {
			if f.set {
				errf("%s is only valid on agent steps", f.field)
			}
		}
	}
	if kind != KindCI {
		ciOnly := []struct {
			field string
			set   bool
		}{
			{"body", s.Body != ""},
			{"status", s.Status != ""},
			{"title", s.Title != ""},
		}
		for _, f := range ciOnly {
			if f.set {
				errf("%s is only valid on ci steps", f.field)
			}
		}
	}

	if kind == KindCI && !slices.Contains(ciActions, s.CI) {
		msg := fmt.Sprintf("unknown ci action %q", s.CI)
		if sug := Suggest(s.CI, ciActions); sug != "" {
			msg += fmt.Sprintf(" — did you mean %q?", sug)
		}
		errf("%s", msg)
	}

	keys := make([]string, 0, len(s.Outputs))
	for k := range s.Outputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !outputTypes[s.Outputs[k]] {
			errf("unknown output type %q for key %q (string|number|boolean|list|object)",
				s.Outputs[k], k)
		}
	}

	if s.Retries < 0 {
		errf("retries must be >= 0")
	}
	if s.Timeout.Duration < 0 {
		errf("timeout must be >= 0")
	}
	if s.Budget.MaxCost < 0 {
		errf("budget.max_cost must be > 0 when set")
	}
	if s.Budget.MaxTokens < 0 {
		errf("budget.max_tokens must be > 0 when set")
	}
	return issues
}
