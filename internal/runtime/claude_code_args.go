package runtime

import "strings"

// claudeCodeToolMap maps Jerry permission nouns to Claude Code tool names.
var claudeCodeToolMap = map[string]string{
	"read":  "Read",
	"write": "Write",
	"edit":  "Edit",
	"bash":  "Bash",
	"grep":  "Grep",
	"find":  "Glob",
	"ls":    "LS",
}

func buildClaudeCodeArgs(inv InvocationSpec) []string {
	args := []string{"-p", inv.Prompt, "--output-format", "json"}
	if inv.Model != "" {
		args = append(args, "--model", inv.Model)
	}

	allowed := claudeCodePermsToTools(inv.Permissions.Allow)
	denied := claudeCodePermsToTools(inv.Permissions.Deny)

	if len(allowed) > 0 {
		args = append(args, "--allowedTools", strings.Join(allowed, ","))
	}
	if len(denied) > 0 {
		args = append(args, "--disallowedTools", strings.Join(denied, ","))
	}

	return args
}

func claudeCodePermsToTools(patterns []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, pat := range patterns {
		noun := pat
		if i := strings.IndexByte(noun, '('); i >= 0 {
			noun = noun[:i]
		}
		noun = strings.TrimSpace(noun)
		tool, ok := claudeCodeToolMap[noun]
		if !ok || seen[tool] {
			continue
		}
		seen[tool] = true
		out = append(out, tool)
	}
	return out
}
