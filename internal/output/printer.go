// Package output handles all CLI output formatting for Motif.
// All human-readable output goes to stderr so that stdout remains
// clean for piping. Uses Unicode symbols for visual clarity.
package output

import (
	"fmt"
	"io"
	"time"
)

// Printer handles all CLI output formatting.
type Printer struct {
	stdout io.Writer
	stderr io.Writer
}

// NewPrinter creates a printer writing to the given streams.
// All human-readable output goes to stderr. stdout is reserved
// for structured data output (for piping).
func NewPrinter(stdout, stderr io.Writer) *Printer {
	return &Printer{
		stdout: stdout,
		stderr: stderr,
	}
}

// Info prints an informational message to stderr.
// Format: "motif: <message>"
func (p *Printer) Info(format string, args ...any) {
	fmt.Fprintf(p.stderr, "motif: "+format+"\n", args...)
}

// StepStart prints the step starting indicator to stderr.
// Format: "  ▸ <name> ..."
func (p *Printer) StepStart(name string) {
	fmt.Fprintf(p.stderr, "  ▸ %s ...\n", name)
}

// StepSuccess prints the step completion indicator to stderr.
// Format: "  ✓ <name> (<duration>)"
func (p *Printer) StepSuccess(name string, duration time.Duration) {
	fmt.Fprintf(p.stderr, "  ✓ %s (%s)\n", name, formatDuration(duration))
}

// StepFailed prints the step failure indicator to stderr.
// Format: "  ✗ <name> — <message>"
func (p *Printer) StepFailed(name string, message string) {
	fmt.Fprintf(p.stderr, "  ✗ %s — %s\n", name, message)
}

// StepSkipped prints the step skipped indicator to stderr.
// Format: "  ⊘ <name> — <reason>"
func (p *Printer) StepSkipped(name string, reason string) {
	fmt.Fprintf(p.stderr, "  ⊘ %s — %s\n", name, reason)
}

// StepOutput prints indented output from a step (e.g., script stderr) to stderr.
// Format: "    <line>"
func (p *Printer) StepOutput(line string) {
	fmt.Fprintf(p.stderr, "    %s\n", line)
}

// Warning prints a warning message to stderr.
// Format: "motif: warning: <message>"
func (p *Printer) Warning(format string, args ...any) {
	fmt.Fprintf(p.stderr, "motif: warning: "+format+"\n", args...)
}

// PipelineComplete prints the final success message to stderr.
// Format: "motif: Pipeline completed in <duration> (run: <runID>)"
func (p *Printer) PipelineComplete(duration time.Duration, runID string) {
	fmt.Fprintf(p.stderr, "motif: Pipeline completed in %s (run: %s)\n",
		formatDuration(duration), runID)
}

// PipelineFailed prints the final failure message to stderr.
// Format: "motif: Pipeline failed at step '<name>': <message>"
//
//	"motif: Run saved: <runID>"
func (p *Printer) PipelineFailed(stepName string, message string, runID string) {
	fmt.Fprintf(p.stderr, "motif: Pipeline failed at step '%s': %s\n", stepName, message)
	fmt.Fprintf(p.stderr, "motif: Run saved: %s\n", runID)
}

// ValidationResult prints the result of validating a pipeline to stderr.
// Format (valid):   "  ✓ <file> — <detail>"
// Format (invalid): "  ✗ <file> — <detail>"
func (p *Printer) ValidationResult(file string, valid bool, detail string) {
	if valid {
		fmt.Fprintf(p.stderr, "  ✓ %s — %s\n", file, detail)
	} else {
		fmt.Fprintf(p.stderr, "  ✗ %s — %s\n", file, detail)
	}
}

// formatDuration formats a duration as a human-readable string.
// Examples: "1.2s", "45.0s", "2m 30s"
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
