// Package output handles all CLI output formatting for Jerry.
// All human-readable output goes to stderr so that stdout remains
// clean for piping. Uses Unicode symbols for visual clarity.
package output

import (
	"fmt"
	"io"
	"time"
)

// Verbosity controls how much detail is printed during execution.
type Verbosity int

const (
	VerbosityQuiet   Verbosity = 0
	VerbosityDefault Verbosity = 1
	VerbosityVerbose Verbosity = 2
)

// Printer handles all CLI output formatting.
type Printer struct {
	stdout    io.Writer
	stderr    io.Writer
	verbosity Verbosity
}

// NewPrinter creates a printer at default verbosity.
func NewPrinter(stdout, stderr io.Writer) *Printer {
	return &Printer{
		stdout:    stdout,
		stderr:    stderr,
		verbosity: VerbosityDefault,
	}
}

// SetVerbosity changes the output verbosity level.
func (p *Printer) SetVerbosity(v Verbosity) {
	p.verbosity = v
}

// Info prints an informational message to stderr.
func (p *Printer) Info(format string, args ...any) {
	if p.verbosity >= VerbosityDefault {
		fmt.Fprintf(p.stderr, "jerry: "+format+"\n", args...)
	}
}

// StepStart prints the step starting indicator.
func (p *Printer) StepStart(name string) {
	if p.verbosity >= VerbosityDefault {
		fmt.Fprintf(p.stderr, "  ▸ %s ...\n", name)
	}
}

// StepSuccess prints the step completion indicator with optional agent metrics.
func (p *Printer) StepSuccess(name string, duration time.Duration, iterations, toolCalls, tokensInput, tokensOutput int) {
	if p.verbosity >= VerbosityDefault {
		if iterations > 0 {
			fmt.Fprintf(p.stderr, "  ✓ %s (%s, %d iter, %d tools, %dk tokens)\n",
				name, formatDuration(duration), iterations, toolCalls,
				(tokensInput+tokensOutput)/1000)
		} else {
			fmt.Fprintf(p.stderr, "  ✓ %s (%s)\n", name, formatDuration(duration))
		}
	}
}

// StepFailed prints the step failure indicator.
func (p *Printer) StepFailed(name, message string) {
	// Always show failures, even in quiet mode.
	fmt.Fprintf(p.stderr, "  ✗ %s — %s\n", name, message)
}

// StepSkipped prints the step skipped indicator.
func (p *Printer) StepSkipped(name, reason string) {
	if p.verbosity >= VerbosityDefault {
		fmt.Fprintf(p.stderr, "  ⊘ %s — %s\n", name, reason)
	}
}

// StepOutput prints indented output from a step.
func (p *Printer) StepOutput(line string) {
	if p.verbosity >= VerbosityDefault {
		fmt.Fprintf(p.stderr, "    %s\n", line)
	}
}

// Warning prints a warning message.
func (p *Printer) Warning(format string, args ...any) {
	// Warnings shown at default and verbose levels.
	if p.verbosity >= VerbosityDefault {
		fmt.Fprintf(p.stderr, "jerry: warning: "+format+"\n", args...)
	}
}

// WorkflowComplete prints the final success message.
func (p *Printer) WorkflowComplete(duration time.Duration, runID string, totalTokens int) {
	if p.verbosity >= VerbosityQuiet {
		if totalTokens > 0 {
			fmt.Fprintf(p.stderr, "jerry: Workflow completed in %s (run: %s, %dk tokens)\n",
				formatDuration(duration), runID, totalTokens/1000)
		} else {
			fmt.Fprintf(p.stderr, "jerry: Workflow completed in %s (run: %s)\n",
				formatDuration(duration), runID)
		}
	}
}

// WorkflowFailed prints the failure message.
func (p *Printer) WorkflowFailed(stepName, message string) {
	fmt.Fprintf(p.stderr, "jerry: Workflow failed at step '%s': %s\n", stepName, message)
}

// ValidationResult prints the result of validating a workflow.
func (p *Printer) ValidationResult(file string, valid bool, detail string) {
	if valid {
		fmt.Fprintf(p.stderr, "  ✓ %s — %s\n", file, detail)
	} else {
		fmt.Fprintf(p.stderr, "  ✗ %s — %s\n", file, detail)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
