package exec

import (
	"fmt"
	"strings"

	"github.com/kilupskalvis/jerry/internal/handoff"
	"github.com/kilupskalvis/jerry/internal/spec"
)

// runCIStep templates the ci action's fields and, in preview mode, prints
// the fully resolved payload instead of calling the platform API. Live
// API calls arrive with citools wiring in phase 4; until then --ci-live
// is a config error rather than a silent no-op.
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
		return ExitConfig, fmt.Errorf("step %q: --ci-live is not wired yet (citools land in phase 4); run without it for preview", step.Name)
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
