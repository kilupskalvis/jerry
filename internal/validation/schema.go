// Package validation provides deep schema checks for workflow and agent configs.
package validation

import "fmt"

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

var workflowFields = []string{"steps", "description"}

var stepFields = []string{"name", "agent", "run", "retries", "timeout"}

var agentFields = []string{"name", "model", "temperature", "max_iterations", "tools", "secrets", "provider", "permissions"}

var validProviders = map[string]bool{"anthropic": true, "openai": true}

// CheckWorkflowFields validates workflow YAML keys and step-level keys.
func CheckWorkflowFields(raw map[string]any) []FieldError {
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

	return errs
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
