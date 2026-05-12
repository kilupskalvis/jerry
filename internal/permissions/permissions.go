// Package permissions provides a layered deny/allow permission system for tool execution.
package permissions

// ToolRule maps a tool name to a list of glob patterns.
type ToolRule struct {
	Tool     string
	Patterns []string
}

// Permissions holds deny and allow rules for tool execution.
type Permissions struct {
	Deny  []ToolRule
	Allow []ToolRule
}

// Denial describes why a tool call was blocked.
type Denial struct {
	Tool    string
	Input   string
	Pattern string
	Source  string
}

// DenyFor returns all deny patterns for a given tool, or nil if none.
func (p Permissions) DenyFor(tool string) []string {
	var patterns []string
	for _, r := range p.Deny {
		if r.Tool == tool {
			patterns = append(patterns, r.Patterns...)
		}
	}
	return patterns
}

// AllowFor returns allow patterns for a given tool, or nil if no allow restriction.
func (p Permissions) AllowFor(tool string) []string {
	var patterns []string
	found := false
	for _, r := range p.Allow {
		if r.Tool == tool {
			found = true
			patterns = append(patterns, r.Patterns...)
		}
	}
	if !found {
		return nil
	}
	return patterns
}

// Merge combines two Permissions layers. Deny rules union. Allow rules intersect.
func (p Permissions) Merge(overlay Permissions) Permissions {
	return Permissions{
		Deny:  mergeToolRules(p.Deny, overlay.Deny),
		Allow: intersectAllowRules(p.Allow, overlay.Allow),
	}
}

func mergeToolRules(base, overlay []ToolRule) []ToolRule {
	byTool := make(map[string][]string)
	for _, r := range base {
		byTool[r.Tool] = append(byTool[r.Tool], r.Patterns...)
	}
	for _, r := range overlay {
		byTool[r.Tool] = append(byTool[r.Tool], r.Patterns...)
	}

	result := make([]ToolRule, 0, len(byTool))
	for tool, patterns := range byTool {
		result = append(result, ToolRule{Tool: tool, Patterns: patterns})
	}
	return result
}

func intersectAllowRules(base, overlay []ToolRule) []ToolRule {
	baseMap := toolRuleMap(base)
	overlayMap := toolRuleMap(overlay)

	allTools := make(map[string]struct{})
	for t := range baseMap {
		allTools[t] = struct{}{}
	}
	for t := range overlayMap {
		allTools[t] = struct{}{}
	}

	var result []ToolRule
	for tool := range allTools {
		basePats, baseExists := baseMap[tool]
		overlayPats, overlayExists := overlayMap[tool]

		switch {
		case baseExists && overlayExists:
			common := intersectStrings(basePats, overlayPats)
			if len(common) > 0 {
				result = append(result, ToolRule{Tool: tool, Patterns: common})
			}
		case baseExists:
			result = append(result, ToolRule{Tool: tool, Patterns: basePats})
		case overlayExists:
			result = append(result, ToolRule{Tool: tool, Patterns: overlayPats})
		}
	}

	return result
}

func toolRuleMap(rules []ToolRule) map[string][]string {
	m := make(map[string][]string)
	for _, r := range rules {
		m[r.Tool] = append(m[r.Tool], r.Patterns...)
	}
	return m
}

func intersectStrings(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, s := range b {
		set[s] = struct{}{}
	}
	var result []string
	for _, s := range a {
		if _, ok := set[s]; ok {
			result = append(result, s)
		}
	}
	return result
}
