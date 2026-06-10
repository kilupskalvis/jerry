package handoff

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

// RunMeta is run-level template state.
type RunMeta struct {
	ID     string
	Cost   float64
	Tokens int64
}

// RunContext is everything templates can reference.
type RunContext struct {
	Trigger trigger.TriggerData
	Run     RunMeta
	Steps   map[string]StepRecord
	Order   []string // step names in execution order, for default context
}

// Resolve substitutes every ${{ }} reference in text against ctx. Unknown
// references are errors — exec fails fast rather than emitting blanks.
func Resolve(text string, ctx *RunContext) (string, error) {
	refs, err := ExtractRefs(text)
	if err != nil {
		return "", err
	}
	if len(refs) == 0 {
		return text, nil
	}

	var b strings.Builder
	last := 0
	for _, r := range refs {
		val, err := resolveRef(r, ctx)
		if err != nil {
			return "", err
		}
		end := strings.Index(text[r.Offset:], "}}") + r.Offset + 2
		b.WriteString(text[last:r.Offset])
		b.WriteString(val)
		last = end
	}
	b.WriteString(text[last:])
	return b.String(), nil
}

func resolveRef(r Ref, ctx *RunContext) (string, error) {
	switch r.Kind {
	case RefTrigger:
		switch r.Key {
		case "intent":
			return ctx.Trigger.Intent, nil
		case "type":
			return ctx.Trigger.Type, nil
		case "source":
			return ctx.Trigger.Source, nil
		}
		return "", fmt.Errorf("unknown trigger field %q", r.Key)

	case RefTriggerRaw:
		return PathLookup(ctx.Trigger.RawPayload, r.Key)

	case RefRun:
		switch r.Key {
		case "id":
			return ctx.Run.ID, nil
		case "cost":
			return strconv.FormatFloat(ctx.Run.Cost, 'f', -1, 64), nil
		case "tokens":
			return strconv.FormatInt(ctx.Run.Tokens, 10), nil
		}
		return "", fmt.Errorf("unknown run field %q", r.Key)

	case RefStepOutput, RefStepOutputs, RefStepDiff, RefStepDiffStat:
		rec, ok := ctx.Steps[r.Step]
		if !ok {
			return "", fmt.Errorf("step %q has no record in this run", r.Step)
		}
		switch r.Kind {
		case RefStepOutput:
			return rec.Output, nil
		case RefStepDiff:
			return rec.Diff, nil
		case RefStepDiffStat:
			return rec.DiffStat, nil
		default:
			v, ok := rec.Outputs[r.Key]
			if !ok {
				return "", fmt.Errorf("step %q produced no output %q", r.Step, r.Key)
			}
			return renderOutput(v), nil
		}
	}
	return "", fmt.Errorf("unhandled ref kind %d", r.Kind)
}

func renderOutput(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		data, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprintf("%v", x)
		}
		return string(data)
	}
}
