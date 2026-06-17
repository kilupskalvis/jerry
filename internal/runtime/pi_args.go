package runtime

import (
	"strings"

	"github.com/kilupskalvis/jerry/internal/spec"
)

// piBuiltinTools is the set of pi tool nouns Jerry permission patterns can
// map onto (pi 0.73.1 built-ins).
var piBuiltinTools = map[string]bool{
	"read": true, "write": true, "edit": true,
	"bash": true, "grep": true, "find": true, "ls": true,
}

// buildArgs constructs the pi CLI argument vector for one invocation:
// flags first, prompt last. The prompt is a positional arg (no shell).
func buildArgs(inv InvocationSpec) []string {
	args := []string{"--print", "--mode", "json"}
	if inv.Model != "" {
		args = append(args, "--model", inv.Model)
	}

	tools := permsToToolNames(inv.Permissions)
	if len(tools) == 0 {
		args = append(args, "--no-tools")
	} else {
		args = append(args, "--tools", strings.Join(tools, ","))
	}

	args = append(args, inv.Prompt)
	return args
}

// permsToToolNames maps allow patterns to pi built-in tool names. Each
// pattern's noun (text before "(") is taken; unknown nouns are dropped;
// the result is deduped, preserving first-seen order. Deny patterns are
// not representable as pi flags and are intentionally ignored here.
func permsToToolNames(p spec.PermissionSet) []string {
	var out []string
	seen := map[string]bool{}
	for _, pat := range p.Allow {
		noun := pat
		if i := strings.IndexByte(noun, '('); i >= 0 {
			noun = noun[:i]
		}
		noun = strings.TrimSpace(noun)
		if !piBuiltinTools[noun] || seen[noun] {
			continue
		}
		seen[noun] = true
		out = append(out, noun)
	}
	return out
}
