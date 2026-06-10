// Package runtime defines the boundary between Jerry and agent runtimes.
// Adapters are process shims: build argv, spawn, parse. No LLM logic, no
// retries (CI's job), no sequencing (also CI's job).
package runtime

import (
	"context"
	"time"

	"github.com/kilupskalvis/jerry/internal/spec"
)

// Adapter invokes an agent runtime for a single step.
type Adapter interface {
	Name() string
	Capabilities() Capabilities
	// Invoke blocks until the runtime exits. ctx cancellation must kill
	// the child process group.
	Invoke(ctx context.Context, inv InvocationSpec) (Result, error)
}

// StreamingAdapter is an optional upgrade interface (http.Flusher pattern):
// adapters that can mirror runtime activity live implement it.
type StreamingAdapter interface {
	Adapter
	InvokeStream(ctx context.Context, inv InvocationSpec, events func(Event)) (Result, error)
}

// Event is one activity line from a streaming runtime.
type Event struct {
	Kind string
	Text string
}

type Capabilities struct {
	StructuredOutput bool
	CostReporting    bool
	Permissions      bool
	Streaming        bool
}

// InvocationSpec is a fully resolved request for one agent step. No
// templates remain in Prompt; Env is an explicit allowlist.
type InvocationSpec struct {
	Prompt       string
	Workdir      string
	Model        string
	Permissions  spec.PermissionSet
	OutputSchema map[string]string
	Env          []string
	Timeout      time.Duration
}

// Result is what came back from the runtime.
type Result struct {
	Text       string
	Outputs    map[string]any
	Usage      *Usage // nil when the runtime cannot report
	Transcript string
}

// Usage is runtime-reported spend. Jerry keeps no price tables.
type Usage struct {
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}
