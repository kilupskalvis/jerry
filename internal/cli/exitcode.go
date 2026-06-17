package cli

import "fmt"

// execExit is returned by commands that must terminate with a specific
// process exit code (jerry exec mirrors the step's code so CI can branch
// on it). main.go honors any error exposing ExitCode() int.
type execExit struct{ code int }

func (e execExit) Error() string { return fmt.Sprintf("step exited with code %d", e.code) }

// ExitCode returns the process exit code this error should produce.
func (e execExit) ExitCode() int { return e.code }
