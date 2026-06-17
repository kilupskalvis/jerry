package exec

import (
	"fmt"
	"strings"

	"github.com/kilupskalvis/jerry/internal/citools"
	"github.com/kilupskalvis/jerry/internal/handoff"
	"github.com/kilupskalvis/jerry/internal/spec"
)

// runCIStep templates the ci action's fields and dispatches to either
// preview mode (print the resolved payload) or live mode (call the
// GitHub API via citools).
func (e *Executor) runCIStep(step *spec.Step, dir *handoff.CtxDir,
	runCtx *handoff.RunContext, live bool) (int, error) {

	fields := map[string]string{}
	for name, raw := range map[string]string{"body": step.Body, "status": step.Status, "title": step.Title} {
		if raw == "" {
			continue
		}
		resolved, err := handoff.Resolve(raw, runCtx)
		if err != nil {
			return ExitConfig, err
		}
		fields[name] = resolved
	}

	if live {
		return e.runCIStepLive(step, dir, fields, runCtx)
	}

	fmt.Fprintf(e.opts.Out, "▸ %s — ci preview: %s\n", step.Name, step.CI)
	for _, key := range []string{"title", "status", "body"} {
		if v, ok := fields[key]; ok {
			fmt.Fprintf(e.opts.Out, "  %s:\n%s\n", key, indent(v, "    "))
		}
	}

	record := fmt.Sprintf("[preview] %s %v", step.CI, fields)
	if err := dir.WriteStep(handoff.StepRecord{Name: step.Name, Output: record}); err != nil {
		return ExitRuntime, err
	}
	fmt.Fprintf(e.opts.Out, "✓ %s (preview)\n", step.Name)
	return ExitOK, nil
}

func (e *Executor) runCIStepLive(step *spec.Step, dir *handoff.CtxDir,
	fields map[string]string, runCtx *handoff.RunContext) (int, error) {

	client := e.opts.CIClient
	if client == nil {
		var err error
		td := runCtx.Trigger
		client, err = citools.NewClient(&td, e.opts.CIConfig)
		if err != nil {
			return ExitConfig, fmt.Errorf("step %q: cannot create GitHub client: %w", step.Name, err)
		}
	}

	fmt.Fprintf(e.opts.Out, "▸ %s — ci live: %s\n", step.Name, step.CI)

	var result string
	var err error
	switch step.CI {
	case "post_pr_comment":
		result, err = client.PostPRComment(fields["body"])
	case "add_check_status":
		result, err = client.AddCheckStatus(step.Name, fields["status"], fields["body"])
	case "create_pull_request":
		result, err = client.CreatePullRequest(e.opts.RepoRoot, fields["title"], fields["body"], "")
	case "post_review_comment":
		return ExitConfig, fmt.Errorf("step %q: post_review_comment requires path and line fields not yet supported; use post_pr_comment for comment-level findings", step.Name)
	default:
		return ExitConfig, fmt.Errorf("step %q: unknown ci action %q", step.Name, step.CI)
	}
	if err != nil {
		return ExitStep, fmt.Errorf("step %q: %s failed: %w", step.Name, step.CI, err)
	}

	if err := dir.WriteStep(handoff.StepRecord{Name: step.Name, Output: result}); err != nil {
		return ExitRuntime, err
	}
	fmt.Fprintf(e.opts.Out, "✓ %s: %s\n", step.Name, result)
	return ExitOK, nil
}

func indent(s, pad string) string {
	var b strings.Builder
	b.WriteString(pad)
	for _, r := range s {
		b.WriteRune(r)
		if r == '\n' {
			b.WriteString(pad)
		}
	}
	return b.String()
}
