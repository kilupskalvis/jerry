package state

import "context"

type logWriterKey struct{}
type stepNameKey struct{}

// WithLogWriter returns a context carrying the given LogWriter.
func WithLogWriter(parent context.Context, lw *LogWriter) context.Context {
	return context.WithValue(parent, logWriterKey{}, lw)
}

// LogWriterFrom extracts the LogWriter from a context, or nil if not present.
func LogWriterFrom(ctx context.Context) *LogWriter {
	lw, _ := ctx.Value(logWriterKey{}).(*LogWriter)
	return lw
}

// WithStepName returns a context carrying the pipeline step name.
// Used by the engine to pass the step name to the agent loop so all
// log entries use a consistent, pipeline-level step identifier.
func WithStepName(parent context.Context, name string) context.Context {
	return context.WithValue(parent, stepNameKey{}, name)
}

// StepNameFrom extracts the pipeline step name from a context, or "" if not set.
func StepNameFrom(ctx context.Context) string {
	name, _ := ctx.Value(stepNameKey{}).(string)
	return name
}
