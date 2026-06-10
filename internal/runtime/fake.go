package runtime

import "context"

// Fake is a scriptable in-memory adapter for tests. It records every
// InvocationSpec and replays scripted results in order.
type Fake struct {
	name        string
	results     []Result
	errs        []error
	Invocations []InvocationSpec
}

// NewFake returns a Fake that answers to name.
func NewFake(name string) *Fake {
	return &Fake{name: name}
}

// Script queues a successful result.
func (f *Fake) Script(r Result) { f.results, f.errs = append(f.results, r), append(f.errs, nil) }

// ScriptErr queues a failed invocation.
func (f *Fake) ScriptErr(err error) {
	f.results, f.errs = append(f.results, Result{}), append(f.errs, err)
}

func (f *Fake) Name() string { return f.name }

func (f *Fake) Capabilities() Capabilities {
	return Capabilities{StructuredOutput: true, CostReporting: true, Permissions: true}
}

func (f *Fake) Invoke(_ context.Context, inv InvocationSpec) (Result, error) {
	f.Invocations = append(f.Invocations, inv)
	i := len(f.Invocations) - 1
	if i >= len(f.results) {
		return Result{Text: "fake: unscripted"}, nil
	}
	return f.results[i], f.errs[i]
}
