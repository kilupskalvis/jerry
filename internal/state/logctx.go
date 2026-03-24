// Context key for passing LogWriter through execution contexts.

package state

import "context"

type logWriterKey struct{}

// WithLogWriter returns a context carrying the given LogWriter.
func WithLogWriter(parent context.Context, lw *LogWriter) context.Context {
	return context.WithValue(parent, logWriterKey{}, lw)
}

// LogWriterFrom extracts the LogWriter from a context, or nil if not present.
func LogWriterFrom(requestCtx context.Context) *LogWriter {
	lw, _ := requestCtx.Value(logWriterKey{}).(*LogWriter)
	return lw
}
