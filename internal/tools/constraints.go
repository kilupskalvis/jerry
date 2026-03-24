// Constraint enforcement for tool arguments (path restrictions, command allowlists).

package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateConstraints checks tool arguments against the tool's constraints.
// Returns an error message string if violated, empty string if valid.
func ValidateConstraints(toolName string, args, constraints map[string]any, repoRoot string) string {
	switch toolName {
	case "write_file":
		return validateWriteFileConstraints(args, constraints, repoRoot)
	case "run_command":
		return validateRunCommandConstraints(args, constraints)
	default:
		return ""
	}
}

// validateWriteFileConstraints checks write_file's restrict_to constraint.
func validateWriteFileConstraints(args, constraints map[string]any, repoRoot string) string {
	restrictToRaw, ok := constraints["restrict_to"]
	if !ok {
		return ""
	}

	restrictTo, ok := toStringSlice(restrictToRaw)
	if !ok {
		return ""
	}

	path, _ := args["path"].(string)
	if path == "" {
		return ""
	}

	absPath := filepath.Join(repoRoot, path)
	absPath, _ = filepath.Abs(absPath)

	for _, allowed := range restrictTo {
		allowedAbs := filepath.Join(repoRoot, allowed)
		allowedAbs, _ = filepath.Abs(allowedAbs)

		// Ensure the allowed path ends with separator for prefix matching.
		if !strings.HasSuffix(allowedAbs, string(filepath.Separator)) {
			allowedAbs += string(filepath.Separator)
		}

		if strings.HasPrefix(absPath, allowedAbs) {
			return ""
		}
	}

	return fmt.Sprintf("write_file blocked: path '%s' is outside allowed directories %v", path, restrictTo)
}

// validateRunCommandConstraints checks run_command's allow/deny constraints.
func validateRunCommandConstraints(args, constraints map[string]any) string {
	command, _ := args["command"].(string)
	if command == "" {
		return ""
	}

	denyRaw, hasDeny := constraints["deny"]
	allowRaw, hasAllow := constraints["allow"]

	if !hasDeny && !hasAllow {
		return "" // No constraints — all commands permitted.
	}

	deny, _ := toStringSlice(denyRaw)
	allow, _ := toStringSlice(allowRaw)

	// Split on shell operators to validate each sub-command independently.
	subCommands := splitShellCommands(command)

	for _, sub := range subCommands {
		trimmed := strings.TrimSpace(sub)
		if trimmed == "" {
			continue
		}

		// Deny is checked first — takes precedence over allow.
		for _, d := range deny {
			if strings.HasPrefix(trimmed, d) {
				return fmt.Sprintf("run_command blocked: command '%s' matches deny rule '%s'", trimmed, d)
			}
		}

		// If allow list exists, command must match at least one entry.
		if hasAllow && len(allow) > 0 {
			matched := false
			for _, a := range allow {
				if strings.HasPrefix(trimmed, a) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Sprintf("run_command blocked: command '%s' does not match any allow rule", trimmed)
			}
		}
	}

	return ""
}

// splitShellCommands splits a command string on unquoted shell operators
// (&&, ||, ;, |) while respecting single and double quotes, escape characters,
// $() subshells, and backtick subshells.
func splitShellCommands(command string) []string {
	var commands []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	subshellDepth := 0

	runes := []rune(command)
	i := 0
	for i < len(runes) {
		ch := runes[i]

		prevCh := rune(0)
		if i > 0 {
			prevCh = runes[i-1]
		}

		nextCh := rune(0)
		if i+1 < len(runes) {
			nextCh = runes[i+1]
		}

		// Handle quoting.
		if ch == '\'' && !inDoubleQuote && !inBacktick && prevCh != '\\' {
			inSingleQuote = !inSingleQuote
			current.WriteRune(ch)
			i++
			continue
		}
		if ch == '"' && !inSingleQuote && !inBacktick && prevCh != '\\' {
			inDoubleQuote = !inDoubleQuote
			current.WriteRune(ch)
			i++
			continue
		}

		// Inside quotes — no operator splitting.
		if inSingleQuote || inDoubleQuote {
			current.WriteRune(ch)
			i++
			continue
		}

		// Backtick tracking.
		if ch == '`' {
			inBacktick = !inBacktick
			current.WriteRune(ch)
			i++
			continue
		}
		if inBacktick {
			current.WriteRune(ch)
			i++
			continue
		}

		// Subshell tracking: $( ... )
		if ch == '$' && nextCh == '(' {
			subshellDepth++
			current.WriteRune(ch)
			current.WriteRune(nextCh)
			i += 2
			continue
		}
		if ch == '(' && subshellDepth > 0 {
			subshellDepth++
			current.WriteRune(ch)
			i++
			continue
		}
		if ch == ')' && subshellDepth > 0 {
			subshellDepth--
			current.WriteRune(ch)
			i++
			continue
		}
		if subshellDepth > 0 {
			current.WriteRune(ch)
			i++
			continue
		}

		// Two-char operators: && and ||
		if ch == '&' && nextCh == '&' {
			flushCommand(&commands, &current)
			i += 2
			continue
		}
		if ch == '|' && nextCh == '|' {
			flushCommand(&commands, &current)
			i += 2
			continue
		}

		// Single-char operators: ; and |
		if ch == ';' {
			flushCommand(&commands, &current)
			i++
			continue
		}
		if ch == '|' {
			flushCommand(&commands, &current)
			i++
			continue
		}

		current.WriteRune(ch)
		i++
	}

	flushCommand(&commands, &current)
	return commands
}

// flushCommand trims the current builder content and appends it to commands
// if non-empty, then resets the builder.
func flushCommand(commands *[]string, current *strings.Builder) {
	s := strings.TrimSpace(current.String())
	if s != "" {
		*commands = append(*commands, s)
	}
	current.Reset()
}

// toStringSlice converts an interface{} (from YAML parsing) to []string.
// Handles both []any (from YAML) and []string forms.
func toStringSlice(v any) ([]string, bool) {
	switch val := v.(type) {
	case []string:
		return val, true
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result, len(result) > 0
	default:
		return nil, false
	}
}
