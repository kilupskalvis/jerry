// Package output handles CLI status output for Jerry. Human-readable
// output goes to stderr so stdout stays clean for piping. Per-step
// execution progress is written directly by the exec package to its own
// writer; this printer covers CLI-level messages (warnings, validation).
package output

import (
	"fmt"
	"io"
)

// Verbosity controls how much detail is printed.
type Verbosity int

const (
	VerbosityQuiet   Verbosity = 0
	VerbosityDefault Verbosity = 1
	VerbosityVerbose Verbosity = 2
)

// Printer formats CLI-level output.
type Printer struct {
	stdout    io.Writer
	stderr    io.Writer
	verbosity Verbosity
}

// NewPrinter creates a printer at default verbosity.
func NewPrinter(stdout, stderr io.Writer) *Printer {
	return &Printer{stdout: stdout, stderr: stderr, verbosity: VerbosityDefault}
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

// Warning prints a warning message to stderr.
func (p *Printer) Warning(format string, args ...any) {
	if p.verbosity >= VerbosityDefault {
		fmt.Fprintf(p.stderr, "jerry: warning: "+format+"\n", args...)
	}
}

// ValidationResult prints the result of validating a workflow.
func (p *Printer) ValidationResult(file string, valid bool, detail string) {
	symbol := "✓"
	if !valid {
		symbol = "✗"
	}
	fmt.Fprintf(p.stderr, "  %s %s — %s\n", symbol, file, detail)
}
