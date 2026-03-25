// Output parsing: extracts and validates JSON from agent responses.

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	jerryErrors "github.com/kilupskalvis/jerry/internal/errors"
)

// ParseOutput attempts to extract structured data from the agent's final response.
// If a schema is provided, it is translated from simplified notation and the output
// is validated against it using full JSON Schema validation.
func ParseOutput(rawOutput string, schema map[string]any) (map[string]any, error) {
	parsed, err := extractJSON(rawOutput)
	if err != nil {
		return nil, jerryErrors.New(jerryErrors.CodeInvalidOutputJSON,
			fmt.Sprintf("agent output is not valid JSON: %s", err))
	}

	result, ok := parsed.(map[string]any)
	if !ok {
		return nil, jerryErrors.New(jerryErrors.CodeInvalidOutputJSON,
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
	jsonSchema, translateErr := TranslateSchema(simplifiedSchema)
	if translateErr != nil {
		fmt.Fprintf(os.Stderr, "jerry: warning: schema translation failed (%s), falling back to key checking\n", translateErr)
		return validateTopLevelKeys(value, simplifiedSchema)
	}

	schemaJSON, marshalErr := json.Marshal(jsonSchema)
	if marshalErr != nil {
		fmt.Fprintf(os.Stderr, "jerry: warning: schema marshal failed (%s), falling back to key checking\n", marshalErr)
		return validateTopLevelKeys(value, simplifiedSchema)
	}

	compiled, compileErr := jsonschema.UnmarshalJSON(strings.NewReader(string(schemaJSON)))
	if compileErr != nil {
		fmt.Fprintf(os.Stderr, "jerry: warning: schema compilation failed (%s), falling back to key checking\n", compileErr)
		return validateTopLevelKeys(value, simplifiedSchema)
	}

	compiler := jsonschema.NewCompiler()
	if addErr := compiler.AddResource("schema.json", compiled); addErr != nil {
		fmt.Fprintf(os.Stderr, "jerry: warning: schema resource failed (%s), falling back to key checking\n", addErr)
		return validateTopLevelKeys(value, simplifiedSchema)
	}

	schema, schemaErr := compiler.Compile("schema.json")
	if schemaErr != nil {
		fmt.Fprintf(os.Stderr, "jerry: warning: schema compile failed (%s), falling back to key checking\n", schemaErr)
		return validateTopLevelKeys(value, simplifiedSchema)
	}

	if validErr := schema.Validate(value); validErr != nil {
		return jerryErrors.New(jerryErrors.CodeOutputSchemaViolation,
			fmt.Sprintf("agent output does not match schema: %s", validErr))
	}

	return nil
}

// validateTopLevelKeys is the fallback: checks that all top-level keys
// defined in the schema exist in the parsed output.
func validateTopLevelKeys(result, schema map[string]any) error {
	var missing []string
	for key := range schema {
		if _, exists := result[key]; !exists {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return jerryErrors.New(jerryErrors.CodeOutputSchemaViolation,
			fmt.Sprintf("agent output missing required keys: %s", strings.Join(missing, ", ")))
	}

	return nil
}

// extractJSON tries multiple strategies to extract JSON from the agent's raw output:
// 1. Direct unmarshal
// 2. Extract from markdown code fences (```json ... ```)
// 3. Extract between first { and last }
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
