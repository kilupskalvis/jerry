package agent_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kilupskalvis/motif/internal/agent"
	motifErrors "github.com/kilupskalvis/motif/internal/errors"
)

func TestParseOutput_ValidJSON(t *testing.T) {
	result, err := agent.ParseOutput(`{"key": "value"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestParseOutput_JSONInText(t *testing.T) {
	result, err := agent.ParseOutput(`Here is the result: {"key": "value"} hope that helps`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestParseOutput_JSONInMarkdownFence(t *testing.T) {
	raw := "Here is the output:\n```json\n{\"key\": \"value\"}\n```\n"
	result, err := agent.ParseOutput(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestParseOutput_InvalidJSON(t *testing.T) {
	_, err := agent.ParseOutput("not json at all", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	var motifErr *motifErrors.Error
	if !errors.As(err, &motifErr) {
		t.Fatalf("expected motif Error, got %T", err)
	}
	if motifErr.Code != motifErrors.CodeInvalidOutputJSON {
		t.Errorf("expected code %q, got %q", motifErrors.CodeInvalidOutputJSON, motifErr.Code)
	}
}

func TestParseOutput_SchemaMatch(t *testing.T) {
	schema := map[string]any{
		"summary": "string",
		"items":   map[string]any{"type": "array"},
	}

	result, err := agent.ParseOutput(`{"summary": "test", "items": [1, 2]}`, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["summary"] != "test" {
		t.Errorf("expected summary=test, got %v", result["summary"])
	}
}

func TestParseOutput_SchemaMismatch(t *testing.T) {
	schema := map[string]any{
		"summary": "string",
		"items":   map[string]any{"type": "array"},
	}

	_, err := agent.ParseOutput(`{"summary": "test"}`, schema)
	if err == nil {
		t.Fatal("expected error for missing schema key")
	}

	var motifErr *motifErrors.Error
	if !errors.As(err, &motifErr) {
		t.Fatalf("expected motif Error, got %T", err)
	}
	if motifErr.Code != motifErrors.CodeOutputSchemaViolation {
		t.Errorf("expected code %q, got %q", motifErrors.CodeOutputSchemaViolation, motifErr.Code)
	}
	if !strings.Contains(motifErr.Message, "items") {
		t.Errorf("error should mention missing key 'items', got: %s", motifErr.Message)
	}
}

func TestParseOutput_NilSchema(t *testing.T) {
	result, err := agent.ParseOutput(`{"anything": "goes"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["anything"] != "goes" {
		t.Errorf("expected anything=goes, got %v", result["anything"])
	}
}

func TestParseOutput_ArrayNotObject(t *testing.T) {
	_, err := agent.ParseOutput(`[1, 2, 3]`, nil)
	if err == nil {
		t.Fatal("expected error for array output")
	}

	var motifErr *motifErrors.Error
	if !errors.As(err, &motifErr) {
		t.Fatalf("expected motif Error, got %T", err)
	}
	if motifErr.Code != motifErrors.CodeInvalidOutputJSON {
		t.Errorf("expected code %q, got %q", motifErrors.CodeInvalidOutputJSON, motifErr.Code)
	}
}

func TestParseOutput_FullSchemaValidation_WrongType(t *testing.T) {
	schema := map[string]any{
		"count": "number",
	}
	_, err := agent.ParseOutput(`{"count": "not a number"}`, schema)
	if err == nil {
		t.Fatal("expected schema violation error for wrong type")
	}

	var motifErr *motifErrors.Error
	if !errors.As(err, &motifErr) {
		t.Fatalf("expected motif Error, got %T", err)
	}
	if motifErr.Code != motifErrors.CodeOutputSchemaViolation {
		t.Errorf("code = %q, want %q", motifErr.Code, motifErrors.CodeOutputSchemaViolation)
	}
}

func TestParseOutput_FullSchemaValidation_NestedObject(t *testing.T) {
	schema := map[string]any{
		"artifacts": map[string]any{
			"type": "array",
			"items": map[string]any{
				"path":    "string",
				"content": "string",
			},
		},
	}
	result, err := agent.ParseOutput(`{"artifacts": [{"path": "a.go", "content": "package a"}]}`, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	artifacts := result["artifacts"].([]any)
	if len(artifacts) != 1 {
		t.Errorf("artifacts length = %d, want 1", len(artifacts))
	}
}

func TestParseOutput_FullSchemaValidation_NestedWrongType(t *testing.T) {
	schema := map[string]any{
		"artifacts": map[string]any{
			"type": "array",
			"items": map[string]any{
				"path":    "string",
				"content": "string",
			},
		},
	}
	// content should be string, not number
	_, err := agent.ParseOutput(`{"artifacts": [{"path": "a.go", "content": 42}]}`, schema)
	if err == nil {
		t.Fatal("expected schema violation for nested wrong type")
	}
}
