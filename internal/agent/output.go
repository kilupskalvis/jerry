// Output parsing: extracts and validates JSON from agent responses.

package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	motifErrors "github.com/kilupskalvis/motif/internal/errors"
)

// ParseOutput attempts to extract structured data from the agent's final response.
//
// Phase 2 validation is minimal: checks that the output is a JSON object with the
// expected top-level keys from the schema. Phase 3 adds full JSON Schema validation.
//
// Returns map[string]any (not any) — agent outputs must be JSON objects. Arrays
// and scalars are rejected. This is stricter than the spec's interface{} return
// type but matches the real-world contract: context keys always map to objects.
func ParseOutput(rawOutput string, schema map[string]any) (map[string]any, error) {
	parsed, err := extractJSON(rawOutput)
	if err != nil {
		return nil, motifErrors.New(motifErrors.CodeInvalidOutputJSON,
			fmt.Sprintf("agent output is not valid JSON: %s", err))
	}

	result, ok := parsed.(map[string]any)
	if !ok {
		return nil, motifErrors.New(motifErrors.CodeInvalidOutputJSON,
			"agent output must be a JSON object, not an array or scalar")
	}

	if schema != nil {
		if validErr := validateTopLevelKeys(result, schema); validErr != nil {
			return nil, validErr
		}
	}

	return result, nil
}

// extractJSON tries multiple strategies to extract JSON from the agent's raw output:
// 1. Direct unmarshal
// 2. Extract from markdown code fences (```json ... ```)
// 3. Extract between first { and last }
func extractJSON(raw string) (any, error) {
	// Strategy 1: direct unmarshal.
	var direct any
	if err := json.Unmarshal([]byte(raw), &direct); err == nil {
		return direct, nil
	}

	// Strategy 2: extract from markdown code fences.
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

	// Also try plain ``` fences.
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

	// Strategy 3: extract between first { and last }.
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

// validateTopLevelKeys checks that all top-level keys defined in the schema
// exist in the parsed output. This is Phase 2's minimal validation — Phase 3
// adds full JSON Schema validation.
func validateTopLevelKeys(result, schema map[string]any) error {
	var missing []string
	for key := range schema {
		if _, exists := result[key]; !exists {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return motifErrors.New(motifErrors.CodeOutputSchemaViolation,
			fmt.Sprintf("agent output missing required keys: %s", strings.Join(missing, ", ")))
	}

	return nil
}
