package runtime

import (
	"fmt"
	"sort"

	"github.com/kilupskalvis/jerry/internal/spec"
)

// Registry resolves runtime names to adapters.
type Registry struct {
	adapters map[string]Adapter
}

// NewRegistry builds a registry from the given adapters.
func NewRegistry(adapters ...Adapter) *Registry {
	m := make(map[string]Adapter, len(adapters))
	for _, a := range adapters {
		m[a.Name()] = a
	}
	return &Registry{adapters: m}
}

// Lookup returns the adapter for name, with a did-you-mean error on miss.
func (r *Registry) Lookup(name string) (Adapter, error) {
	if a, ok := r.adapters[name]; ok {
		return a, nil
	}
	names := make([]string, 0, len(r.adapters))
	for n := range r.adapters {
		names = append(names, n)
	}
	sort.Strings(names)
	msg := fmt.Sprintf("unknown runtime %q (available: %v)", name, names)
	if sug := spec.Suggest(name, names); sug != "" {
		msg += fmt.Sprintf(" — did you mean %q?", sug)
	}
	return nil, fmt.Errorf("%s", msg)
}
