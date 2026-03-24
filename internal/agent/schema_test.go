package agent

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestTranslateSchema_SimpleFields(t *testing.T) {
	simplified := map[string]any{
		"summary": "string",
		"count":   "number",
		"valid":   "boolean",
	}
	schema, err := TranslateSchema(simplified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("root type = %v, want 'object'", schema["type"])
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required field missing or wrong type")
	}
	sort.Strings(required)
	if len(required) != 3 {
		t.Errorf("required has %d entries, want 3", len(required))
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties missing or wrong type")
	}

	for _, tc := range []struct{ key, wantType string }{
		{"summary", "string"},
		{"count", "number"},
		{"valid", "boolean"},
	} {
		prop, ok := props[tc.key].(map[string]any)
		if !ok {
			t.Errorf("property %q missing", tc.key)
			continue
		}
		if prop["type"] != tc.wantType {
			t.Errorf("%s type = %v, want %q", tc.key, prop["type"], tc.wantType)
		}
	}
}

func TestTranslateSchema_ArrayOfStrings(t *testing.T) {
	simplified := map[string]any{
		"tags": map[string]any{
			"type":  "array",
			"items": "string",
		},
	}
	schema, err := TranslateSchema(simplified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	tagsProp := props["tags"].(map[string]any)
	if tagsProp["type"] != "array" {
		t.Errorf("tags type = %v, want 'array'", tagsProp["type"])
	}
	items := tagsProp["items"].(map[string]any)
	if items["type"] != "string" {
		t.Errorf("tags items type = %v, want 'string'", items["type"])
	}
}

func TestTranslateSchema_ArrayOfObjects(t *testing.T) {
	simplified := map[string]any{
		"artifacts": map[string]any{
			"type": "array",
			"items": map[string]any{
				"path":    "string",
				"content": "string",
			},
		},
	}
	schema, err := TranslateSchema(simplified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	artProp := props["artifacts"].(map[string]any)
	items := artProp["items"].(map[string]any)
	if items["type"] != "object" {
		t.Errorf("items type = %v, want 'object'", items["type"])
	}
	itemProps := items["properties"].(map[string]any)
	pathProp := itemProps["path"].(map[string]any)
	if pathProp["type"] != "string" {
		t.Errorf("path type = %v, want 'string'", pathProp["type"])
	}
}

func TestTranslateSchema_AlreadyJSONSchema(t *testing.T) {
	fullSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []any{"name"},
	}
	schema, err := TranslateSchema(fullSchema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should pass through unchanged.
	if schema["type"] != "object" {
		t.Errorf("type = %v, want 'object'", schema["type"])
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["name"]; !ok {
		t.Error("properties should contain 'name'")
	}
}

func TestTranslateSchema_RoundTripsAsValidJSON(t *testing.T) {
	simplified := map[string]any{
		"artifacts": map[string]any{
			"type": "array",
			"items": map[string]any{
				"path":    "string",
				"content": "string",
			},
		},
		"decisions_log": map[string]any{
			"type":  "array",
			"items": "string",
		},
	}
	schema, err := TranslateSchema(simplified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, jsonErr := json.Marshal(schema)
	if jsonErr != nil {
		t.Fatalf("schema is not valid JSON: %v", jsonErr)
	}
}

func TestTranslateSchema_UnknownType(t *testing.T) {
	simplified := map[string]any{
		"field": "unicorn",
	}
	_, err := TranslateSchema(simplified)
	if err == nil {
		t.Fatal("expected error for unknown type 'unicorn'")
	}
}
