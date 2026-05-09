// Package errors defines error types and codes used throughout Jerry.
package errors

import "fmt"

const (
	// Workflow and configuration errors.
	CodeInvalidWorkflow    = "INVALID_WORKFLOW"
	CodeWorkflowNotFound   = "WORKFLOW_NOT_FOUND"
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

	// Workflow resume errors.
	CodeRunNotFound     = "RUN_NOT_FOUND"
	CodeRunNotResumable = "RUN_NOT_RESUMABLE"
	CodeWorkflowChanged = "WORKFLOW_CHANGED"

	// Git tool errors.
	CodeGitNotAvailable = "GIT_NOT_AVAILABLE"

	// Configuration errors.
	CodeConfigInvalid = "CONFIG_INVALID"
)

// Exit codes per spec: 1=step failure, 2=config error, 3=runtime error.
const (
	ExitStepFailure = 1
	ExitConfig      = 2
	ExitRuntime     = 3
)

var exitCodes = map[string]int{
	// Config errors (exit 2).
	CodeInvalidWorkflow:  ExitConfig,
	CodeWorkflowNotFound: ExitConfig,
	CodeJerryDirNotFound: ExitConfig,
	CodeJerryDirExists:   ExitConfig,
	CodeConfigInvalid:    ExitConfig,
	CodeAgentLoadFailed:  ExitConfig,
	CodeToolNotFound:     ExitConfig,
	CodeRunNotFound:      ExitConfig,
	CodeRunNotResumable:  ExitConfig,
	CodeWorkflowChanged:  ExitConfig,

	// Runtime errors (exit 3).
	CodeLLMAuthFailed:      ExitRuntime,
	CodeLLMRateLimited:     ExitRuntime,
	CodeLLMServerError:     ExitRuntime,
	CodeLLMCallFailed:      ExitRuntime,
	CodeStateWriteFailed:   ExitRuntime,
	CodeContextWriteDenied: ExitRuntime,
	CodeGitNotAvailable:    ExitRuntime,
}

// Error is the standard error type for Jerry.
type Error struct {
	Code    string
	Message string
	Step    string
	Cause   error
}

// Error returns a human-readable string representation.
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

// Unwrap returns the underlying cause error.
func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new Error with the given code and message.
func New(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap creates a new Error wrapping an existing error.
func Wrap(code, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// ExitCode returns the CLI exit code for this error.
// Defaults to ExitStepFailure (1) for unrecognized codes.
func (e *Error) ExitCode() int {
	if code, ok := exitCodes[e.Code]; ok {
		return code
	}
	return ExitStepFailure
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
