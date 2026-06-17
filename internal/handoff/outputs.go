package handoff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// StructuredOutputDirective returns prompt text instructing the agent to
// finish with a JSON object carrying the declared output keys. Empty when
// no outputs are declared. Keys are listed in sorted order for determinism.
func StructuredOutputDirective(outputs map[string]string) string {
	if len(outputs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(outputs))
	for k := range outputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("\n\n---\n\nWhen finished, output a single JSON object on its own line with exactly these keys:\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "- %q (%s)\n", k, outputs[k])
	}
	b.WriteString("Output the JSON object last; surrounding prose is allowed but the object must be valid JSON.")
	return b.String()
}

// ParseStructuredText extracts a JSON object from an agent's free-text
// output: it strips ```json fences and scans for the outermost {...}
// object, returning it decoded.
func ParseStructuredText(text string) (map[string]any, error) {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")

	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object found in agent output")
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(s[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("agent output is not valid JSON: %w", err)
	}
	return out, nil
}
