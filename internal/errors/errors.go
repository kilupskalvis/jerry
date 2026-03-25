// Package errors defines the standard error types and codes used throughout Jerry.
// Every component returns errors using these types, enabling programmatic error
// handling via codes and human-readable messages.
package errors

import "fmt"

// Machine-readable error codes. Use these with [New] and [Wrap] to create
// errors that callers can match programmatically via the Code field.
const (
	// Pipeline and configuration errors.
	CodeInvalidPipeline    = "INVALID_PIPELINE"
	CodePipelineNotFound   = "PIPELINE_NOT_FOUND"
	CodeJerryDirNotFound   = "JERRY_DIR_NOT_FOUND"
	CodeJerryDirExists     = "JERRY_DIR_EXISTS"
	CodeContextWriteDenied = "CONTEXT_WRITE_DENIED"
	CodeStateWriteFailed   = "STATE_WRITE_FAILED"

	// Script execution errors.
	CodeScriptFailed  = "SCRIPT_FAILED"
	CodeScriptTimeout = "SCRIPT_TIMEOUT"

	// Agent execution errors.
	CodeAgentLoadFailed    = "AGENT_LOAD_FAILED"
	CodeAgentMaxIterations = "AGENT_MAX_ITERATIONS"

	// LLM provider errors.
	CodeLLMAuthFailed  = "LLM_AUTH_FAILED"
	CodeLLMRateLimited = "LLM_RATE_LIMITED"
	CodeLLMServerError = "LLM_SERVER_ERROR"
	CodeLLMCallFailed  = "LLM_CALL_FAILED"

	// Tool errors.
	CodeToolNotFound            = "TOOL_NOT_FOUND"
	CodeToolConstraintViolation = "TOOL_CONSTRAINT_VIOLATION"

	// Output parsing errors.
	CodeInvalidOutputJSON     = "INVALID_OUTPUT_JSON"
	CodeOutputSchemaViolation = "OUTPUT_SCHEMA_VIOLATION"

	// Context compaction errors.
	CodeCompactionFailed = "COMPACTION_FAILED"
	CodeContextTooLong   = "CONTEXT_TOO_LONG"

	// Pipeline resume errors.
	CodeRunNotFound     = "RUN_NOT_FOUND"
	CodeRunNotResumable = "RUN_NOT_RESUMABLE"
	CodePipelineChanged = "PIPELINE_CHANGED"

	// Git tool errors.
	CodeGitNotAvailable = "GIT_NOT_AVAILABLE"

	// Configuration errors.
	CodeConfigInvalid = "CONFIG_INVALID"
)

// Error is the standard error type returned by all Jerry components.
// It implements the error interface and supports unwrapping via errors.Unwrap.
type Error struct {
	// Code is a machine-readable error code (one of the Code* constants).
	Code string

	// Message is a human-readable description of what went wrong.
	Message string

	// Step is the pipeline step that caused this error, empty if not step-related.
	Step string

	// Cause is the underlying error, nil if none.
	Cause error
}

// Error returns a human-readable string representation of the error.
// Format varies based on which fields are set:
//   - "CODE: message"
//   - "CODE [step: name]: message"
//   - "CODE: message: cause"
//   - "CODE [step: name]: message: cause"
func (e *Error) Error() string {
	var prefix string
	if e.Step != "" {
		prefix = fmt.Sprintf("%s [step: %s]", e.Code, e.Step)
	} else {
		prefix = e.Code
	}

	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %s", prefix, e.Message, e.Cause.Error())
	}
	return fmt.Sprintf("%s: %s", prefix, e.Message)
}

// Unwrap returns the underlying cause error, or nil if none.
// This enables use with errors.Is and errors.As from the standard library.
func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new Error with the given code and message.
// The Step and Cause fields are left empty.
func New(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap creates a new Error wrapping an existing error with additional context.
// The wrapped error is accessible via Unwrap().
func Wrap(code, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// WithStep returns a copy of the error with the Step field set.
// The original error is not modified.
func (e *Error) WithStep(step string) *Error {
	return &Error{
		Code:    e.Code,
		Message: e.Message,
		Step:    step,
		Cause:   e.Cause,
	}
}
