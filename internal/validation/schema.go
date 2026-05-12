// Package validation provides deep schema checks for workflow and agent configs.
package validation

import (
	"fmt"
	"slices"

	"github.com/kilupskalvis/jerry/internal/hooks"
)

// FieldError describes a schema validation problem.
type FieldError struct {
	Field      string
	Message    string
	Suggestion string
}

func (e FieldError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("unknown field %q (did you mean %q?)", e.Field, e.Suggestion)
	}
	return e.Message
}

var workflowFields = []string{"steps", "description", "hooks"}

var stepFields = []string{"name", "agent", "run", "retries", "timeout"}

var agentFields = []string{"name", "model", "temperature", "max_iterations", "tools", "secrets", "provider", "permissions"}

var validProviders = map[string]bool{"anthropic": true, "openai": true}

// CheckWorkflowFields validates workflow YAML keys, step-level keys, and hooks.
// knownTools is used to validate tool names in hook filters (pass nil to skip).
func CheckWorkflowFields(raw map[string]any, knownTools []string) []FieldError {
	var errs []FieldError

	errs = append(errs, checkUnknownFields(raw, workflowFields, "workflow")...)

	steps, ok := raw["steps"]
	if !ok {
		return errs
	}

	stepList, ok := steps.([]any)
	if !ok {
		return errs
	}

	for i, s := range stepList {
		stepMap, ok := s.(map[string]any)
		if !ok {
			continue
		}
		errs = append(errs, checkUnknownFields(stepMap, stepFields, fmt.Sprintf("step %d", i+1))...)
	}

	if hooksRaw, ok := raw["hooks"]; ok {
		errs = append(errs, checkHooks(hooksRaw, knownTools)...)
	}

	return errs
}

var toolEvents = map[string]bool{
	hooks.BeforeToolCall: true,
	hooks.AfterToolCall:  true,
}

func checkHooks(raw any, knownTools []string) []FieldError {
	hooksMap, ok := raw.(map[string]any)
	if !ok {
		return []FieldError{{
			Field:   "hooks",
			Message: "\"hooks\" must be a map of event names to hook lists",
		}}
	}

	var errs []FieldError

	for event, defs := range hooksMap {
		if !isValidEvent(event) {
			fe := FieldError{Field: event}
			if suggestion := Suggest(event, hooks.ValidEvents); suggestion != "" {
				fe.Suggestion = suggestion
			} else {
				fe.Message = fmt.Sprintf("unknown hook event %q", event)
			}
			errs = append(errs, fe)
			continue
		}

		defList, ok := defs.([]any)
		if !ok {
			errs = append(errs, FieldError{
				Field:   event,
				Message: fmt.Sprintf("hooks.%s must be a list", event),
			})
			continue
		}

		for i, def := range defList {
			defMap, ok := def.(map[string]any)
			if !ok {
				errs = append(errs, FieldError{
					Field:   event,
					Message: fmt.Sprintf("hooks.%s[%d] must be an object with \"run\" field", event, i),
				})
				continue
			}

			if _, hasRun := defMap["run"]; !hasRun {
				errs = append(errs, FieldError{
					Field:   event,
					Message: fmt.Sprintf("hooks.%s[%d]: missing required field \"run\"", event, i),
				})
			}

			if toolsRaw, hasTools := defMap["tools"]; hasTools {
				if !toolEvents[event] {
					errs = append(errs, FieldError{
						Field:   event,
						Message: fmt.Sprintf("hooks.%s[%d]: \"tools\" filter is only valid on before_tool_call and after_tool_call", event, i),
					})
				} else if knownTools != nil {
					errs = append(errs, checkHookToolNames(event, i, toolsRaw, knownTools)...)
				}
			}
		}
	}

	return errs
}

func checkHookToolNames(event string, hookIdx int, toolsRaw any, knownTools []string) []FieldError {
	toolsList, ok := toolsRaw.([]any)
	if !ok {
		return nil
	}

	knownSet := make(map[string]bool, len(knownTools))
	for _, t := range knownTools {
		knownSet[t] = true
	}

	var errs []FieldError
	for _, t := range toolsList {
		name, ok := t.(string)
		if !ok {
			continue
		}
		if !knownSet[name] {
			errs = append(errs, FieldError{
				Field:   event,
				Message: fmt.Sprintf("hooks.%s[%d]: unknown tool %q in tools filter", event, hookIdx, name),
			})
		}
	}
	return errs
}

func isValidEvent(event string) bool {
	return slices.Contains(hooks.ValidEvents, event)
}

// CheckAgentFields validates agent frontmatter keys and types.
func CheckAgentFields(raw map[string]any) []FieldError {
	var errs []FieldError
	errs = append(errs, checkUnknownFields(raw, agentFields, "agent")...)
	errs = append(errs, checkAgentTypes(raw)...)
	return errs
}

func checkUnknownFields(raw map[string]any, known []string, context string) []FieldError {
	knownSet := make(map[string]bool, len(known))
	for _, k := range known {
		knownSet[k] = true
	}

	var errs []FieldError
	for key := range raw {
		if knownSet[key] {
			continue
		}
		fe := FieldError{Field: key}
		if suggestion := Suggest(key, known); suggestion != "" {
			fe.Suggestion = suggestion
		} else {
			fe.Message = fmt.Sprintf("unknown field %q in %s", key, context)
		}
		errs = append(errs, fe)
	}

	return errs
}

func checkAgentTypes(raw map[string]any) []FieldError {
	var errs []FieldError

	if v, ok := raw["temperature"]; ok {
		switch v.(type) {
		case float64, int:
		default:
			errs = append(errs, FieldError{
				Field:   "temperature",
				Message: fmt.Sprintf("\"temperature\" must be a number, got %T", v),
			})
		}
	}

	if v, ok := raw["max_iterations"]; ok {
		switch v.(type) {
		case int:
		default:
			errs = append(errs, FieldError{
				Field:   "max_iterations",
				Message: fmt.Sprintf("\"max_iterations\" must be an integer, got %T", v),
			})
		}
	}

	if v, ok := raw["provider"]; ok {
		if s, ok := v.(string); ok {
			if !validProviders[s] {
				errs = append(errs, FieldError{
					Field:   "provider",
					Message: fmt.Sprintf("\"provider\" must be \"anthropic\" or \"openai\", got %q", s),
				})
			}
		}
	}

	if v, ok := raw["tools"]; ok {
		if _, ok := v.([]any); !ok {
			errs = append(errs, FieldError{
				Field:   "tools",
				Message: fmt.Sprintf("\"tools\" must be a list, got %T", v),
			})
		}
	}

	return errs
}
