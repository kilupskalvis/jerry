// Package errors defines the standard error types and codes used throughout Motif.
// Every component returns errors using these types, enabling programmatic error
// handling via codes and human-readable messages.
package errors

import "fmt"

// Error code constants. Each code maps to a category of failure.
const (
	CodeScriptFailed       = "SCRIPT_FAILED"
	CodeScriptTimeout      = "SCRIPT_TIMEOUT"
	CodeAgentNotSupported  = "AGENT_NOT_SUPPORTED"
	CodeInvalidPipeline    = "INVALID_PIPELINE"
	CodePipelineNotFound   = "PIPELINE_NOT_FOUND"
	CodeMotifDirNotFound   = "MOTIF_DIR_NOT_FOUND"
	CodeMotifDirExists     = "MOTIF_DIR_EXISTS"
	CodeContextWriteDenied = "CONTEXT_WRITE_DENIED"
	CodeInvalidOutputJSON  = "INVALID_OUTPUT_JSON"
	CodeStateWriteFailed   = "STATE_WRITE_FAILED"
)

// Error is the standard error type returned by all Motif components.
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
func New(code string, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap creates a new Error wrapping an existing error with additional context.
// The wrapped error is accessible via Unwrap().
func Wrap(code string, message string, cause error) *Error {
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
