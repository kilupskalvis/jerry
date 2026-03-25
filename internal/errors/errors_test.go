package errors_test

import (
	"errors"
	"fmt"
	"testing"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
	}{
		{
			name:    "script failed error",
			code:    jerrerr.CodeScriptFailed,
			message: "script exited with code 1",
		},
		{
			name:    "pipeline not found",
			code:    jerrerr.CodePipelineNotFound,
			message: "pipeline 'feature' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jerrerr.New(tt.code, tt.message)

			if err.Code != tt.code {
				t.Errorf("Code = %q, want %q", err.Code, tt.code)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.Step != "" {
				t.Errorf("Step = %q, want empty", err.Step)
			}
			if err.Cause != nil {
				t.Error("Cause should be nil")
			}
		})
	}
}

func TestWrap(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := jerrerr.Wrap(jerrerr.CodeScriptFailed, "script failed", cause)

	if err.Code != jerrerr.CodeScriptFailed {
		t.Errorf("Code = %q, want %q", err.Code, jerrerr.CodeScriptFailed)
	}
	if err.Message != "script failed" {
		t.Errorf("Message = %q, want %q", err.Message, "script failed")
	}
	if !errors.Is(err.Cause, cause) {
		t.Error("Cause should be the wrapped error")
	}

	// Unwrap should return the cause
	unwrapped := errors.Unwrap(err)
	if !errors.Is(unwrapped, cause) {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestWithStep(t *testing.T) {
	original := jerrerr.New(jerrerr.CodeScriptFailed, "failed")
	withStep := original.WithStep("generate")

	// withStep should have the step set
	if withStep.Step != "generate" {
		t.Errorf("Step = %q, want %q", withStep.Step, "generate")
	}

	// original should be unchanged (WithStep returns a copy)
	if original.Step != "" {
		t.Errorf("original.Step = %q, want empty (should be immutable)", original.Step)
	}

	// other fields should be preserved
	if withStep.Code != original.Code {
		t.Errorf("Code changed: got %q, want %q", withStep.Code, original.Code)
	}
	if withStep.Message != original.Message {
		t.Errorf("Message changed: got %q, want %q", withStep.Message, original.Message)
	}
}

func TestError_ErrorString(t *testing.T) {
	tests := []struct {
		name     string
		err      *jerrerr.Error
		expected string
	}{
		{
			name:     "without step",
			err:      jerrerr.New(jerrerr.CodeScriptFailed, "script exited with code 1"),
			expected: "SCRIPT_FAILED: script exited with code 1",
		},
		{
			name:     "with step",
			err:      jerrerr.New(jerrerr.CodeScriptFailed, "script exited with code 1").WithStep("validate"),
			expected: "SCRIPT_FAILED [step: validate]: script exited with code 1",
		},
		{
			name:     "with cause",
			err:      jerrerr.Wrap(jerrerr.CodeScriptTimeout, "timed out", fmt.Errorf("deadline exceeded")),
			expected: "SCRIPT_TIMEOUT: timed out: deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestError_UnwrapNil(t *testing.T) {
	err := jerrerr.New(jerrerr.CodeScriptFailed, "no cause")
	if errors.Unwrap(err) != nil {
		t.Error("Unwrap() should return nil when there is no cause")
	}
}

func TestError_ImplementsErrorInterface(t *testing.T) {
	var _ error = (*jerrerr.Error)(nil)
}
