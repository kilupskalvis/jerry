package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

// ParseOutput extracts and validates JSON from the agent's response.
func ParseOutput(rawOutput string, schema map[string]any) (map[string]any, error) {
	parsed, err := extractJSON(rawOutput)
	if err != nil {
		return nil, jerrerr.New(jerrerr.CodeInvalidOutputJSON,
			fmt.Sprintf("agent output is not valid JSON: %s", err))
	}

	result, ok := parsed.(map[string]any)
	if !ok {
		return nil, jerrerr.New(jerrerr.CodeInvalidOutputJSON,
			"agent output must be a JSON object, not an array or scalar")
	}

	if schema != nil {
		if validErr := validateAgainstSchema(result, schema); validErr != nil {
			return nil, validErr
		}
	}

	return result, nil
}

// validateAgainstSchema translates the simplified schema, compiles it, and
// validates the parsed output. Falls back to top-level key checking if
// schema translation or compilation fails.
func validateAgainstSchema(value, simplifiedSchema map[string]any) error {
	schema, err := compileSchema(simplifiedSchema)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jerry: warning: %s, falling back to key checking\n", err)
		return validateTopLevelKeys(value, simplifiedSchema)
	}

	if validErr := schema.Validate(value); validErr != nil {
		return jerrerr.New(jerrerr.CodeOutputSchemaViolation,
			fmt.Sprintf("agent output does not match schema: %s", validErr))
	}
	return nil
}

func compileSchema(simplifiedSchema map[string]any) (*jsonschema.Schema, error) {
	jsonSchema, err := TranslateSchema(simplifiedSchema)
	if err != nil {
		return nil, fmt.Errorf("schema translation failed (%w)", err)
	}

	schemaJSON, err := json.Marshal(jsonSchema)
	if err != nil {
		return nil, fmt.Errorf("schema marshal failed (%w)", err)
	}

	compiled, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schemaJSON)))
	if err != nil {
		return nil, fmt.Errorf("schema compilation failed (%w)", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", compiled); err != nil {
		return nil, fmt.Errorf("schema resource failed (%w)", err)
	}

	return compiler.Compile("schema.json")
}

// validateTopLevelKeys checks that all schema keys exist in the output.
func validateTopLevelKeys(result, schema map[string]any) error {
	var missing []string
	for key := range schema {
		if _, exists := result[key]; !exists {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return jerrerr.New(jerrerr.CodeOutputSchemaViolation,
			fmt.Sprintf("agent output missing required keys: %s", strings.Join(missing, ", ")))
	}

	return nil
}

// extractJSON tries direct unmarshal, markdown fences, then brace extraction.
func extractJSON(raw string) (any, error) {
	var direct any
	if err := json.Unmarshal([]byte(raw), &direct); err == nil {
		return direct, nil
	}

	if idx := strings.Index(raw, "```json"); idx != -1 {
		start := idx + len("```json")
		end := strings.Index(raw[start:], "```")
		if end != -1 {
			jsonStr := strings.TrimSpace(raw[start : start+end])
			var fenced any
			if err := json.Unmarshal([]byte(jsonStr), &fenced); err == nil {
				return fenced, nil
			}
		}
	}

	if idx := strings.Index(raw, "```\n"); idx != -1 {
		start := idx + len("```\n")
		end := strings.Index(raw[start:], "```")
		if end != -1 {
			jsonStr := strings.TrimSpace(raw[start : start+end])
			var fenced any
			if err := json.Unmarshal([]byte(jsonStr), &fenced); err == nil {
				return fenced, nil
			}
		}
	}

	firstBrace := strings.Index(raw, "{")
	lastBrace := strings.LastIndex(raw, "}")
	if firstBrace != -1 && lastBrace > firstBrace {
		jsonStr := raw[firstBrace : lastBrace+1]
		var extracted any
		if err := json.Unmarshal([]byte(jsonStr), &extracted); err == nil {
			return extracted, nil
		}
	}

	return nil, fmt.Errorf("no valid JSON found in output")
}
