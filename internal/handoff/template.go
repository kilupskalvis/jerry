// Package handoff implements the context contract between steps: the
// ${{ }} template grammar now; the context directory layout and resolution
// engine arrive with the execution plane.
package handoff

import (
	"fmt"
	"strings"
)

// RefKind classifies a template reference.
type RefKind int

const (
	RefTrigger      RefKind = iota // trigger.intent|type|source
	RefTriggerRaw                  // trigger.raw.<dot.path>
	RefStepOutput                  // steps.<name>.output
	RefStepOutputs                 // steps.<name>.outputs.<key>
	RefStepDiff                    // steps.<name>.diff
	RefStepDiffStat                // steps.<name>.diff_stat
	RefRun                         // run.id|cost|tokens
)

// Ref is one parsed ${{ }} reference.
type Ref struct {
	Raw    string
	Kind   RefKind
	Step   string
	Key    string
	Offset int
}

var (
	triggerFields = map[string]bool{"intent": true, "type": true, "source": true}
	runFields     = map[string]bool{"id": true, "cost": true, "tokens": true}
)

// ExtractRefs scans text and parses every ${{ }} reference. The first
// malformed reference aborts with an error carrying its byte offset.
func ExtractRefs(text string) ([]Ref, error) {
	var refs []Ref
	for i := 0; i < len(text); {
		start := strings.Index(text[i:], "${{")
		if start == -1 {
			break
		}
		start += i
		end := strings.Index(text[start:], "}}")
		if end == -1 {
			return nil, fmt.Errorf("offset %d: unterminated ${{", start)
		}
		end += start

		expr := strings.TrimSpace(text[start+3 : end])
		ref, err := parseExpr(expr)
		if err != nil {
			return nil, fmt.Errorf("offset %d: %w", start, err)
		}
		ref.Offset = start
		refs = append(refs, ref)
		i = end + 2
	}
	return refs, nil
}

func parseExpr(expr string) (Ref, error) {
	if expr == "" {
		return Ref{}, fmt.Errorf("empty template expression")
	}
	ref := Ref{Raw: expr}
	parts := strings.SplitN(expr, ".", 2)
	rest := ""
	if len(parts) == 2 {
		rest = parts[1]
	}

	switch parts[0] {
	case "trigger":
		if path, ok := strings.CutPrefix(rest, "raw."); ok {
			if path == "" {
				return Ref{}, fmt.Errorf("trigger.raw requires a path")
			}
			ref.Kind, ref.Key = RefTriggerRaw, path
			return ref, nil
		}
		if !triggerFields[rest] {
			return Ref{}, fmt.Errorf("unknown trigger field %q (intent|type|source|raw.<path>)", rest)
		}
		ref.Kind, ref.Key = RefTrigger, rest
		return ref, nil

	case "run":
		if !runFields[rest] {
			return Ref{}, fmt.Errorf("unknown run field %q (id|cost|tokens)", rest)
		}
		ref.Kind, ref.Key = RefRun, rest
		return ref, nil

	case "steps":
		segs := strings.SplitN(rest, ".", 2)
		if len(segs) < 2 || segs[0] == "" {
			return Ref{}, fmt.Errorf("expected steps.<name>.output|outputs.<key>|diff|diff_stat, got %q", expr)
		}
		ref.Step = segs[0]
		attr := segs[1]
		switch {
		case attr == "output":
			ref.Kind = RefStepOutput
		case attr == "diff":
			ref.Kind = RefStepDiff
		case attr == "diff_stat":
			ref.Kind = RefStepDiffStat
		case strings.HasPrefix(attr, "outputs."):
			key, _ := strings.CutPrefix(attr, "outputs.")
			if key == "" {
				return Ref{}, fmt.Errorf("steps.%s.outputs requires a key", ref.Step)
			}
			ref.Kind, ref.Key = RefStepOutputs, key
		default:
			return Ref{}, fmt.Errorf("unknown step attribute %q (output|outputs.<key>|diff|diff_stat)", attr)
		}
		return ref, nil

	default:
		return Ref{}, fmt.Errorf("unknown reference root %q (trigger|steps|run)", parts[0])
	}
}
