package spec

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
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
	return issues
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
