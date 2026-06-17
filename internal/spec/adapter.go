package spec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// AdapterSpec is a custom runtime adapter declared in .jerry/adapters/*.yaml.
type AdapterSpec struct {
	Name         string              `yaml:"name"`
	Command      string              `yaml:"command"`
	Prompt       string              `yaml:"prompt"`
	Parse        ParseSpec           `yaml:"parse"`
	Capabilities AdapterCapabilities `yaml:"capabilities"`
}

// ParseSpec defines dot-path expressions for extracting fields from
// the runtime's JSON stdout. Uses the same grammar as handoff.PathLookup.
type ParseSpec struct {
	Text         string `yaml:"text"`
	Cost         string `yaml:"cost,omitempty"`
	InputTokens  string `yaml:"input_tokens,omitempty"`
	OutputTokens string `yaml:"output_tokens,omitempty"`
}

// AdapterCapabilities declares what the custom runtime supports.
type AdapterCapabilities struct {
	StructuredOutput bool `yaml:"structured_output"`
	CostReporting    bool `yaml:"cost_reporting"`
	Permissions      bool `yaml:"permissions"`
}

// LoadAdapters reads .jerry/adapters/*.yaml and returns parsed specs,
// sorted by name. A missing adapters/ directory is not an error.
func LoadAdapters(jerryDir string) ([]AdapterSpec, error) {
	dir := filepath.Join(jerryDir, "adapters")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading adapters directory: %w", err)
	}

	var adapters []AdapterSpec
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("reading adapter %s: %w", e.Name(), readErr)
		}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		var a AdapterSpec
		if decErr := dec.Decode(&a); decErr != nil {
			return nil, fmt.Errorf("parsing adapter %s: %w", e.Name(), decErr)
		}
		if a.Name == "" {
			return nil, fmt.Errorf("adapter %s: name is required", e.Name())
		}
		if a.Command == "" {
			return nil, fmt.Errorf("adapter %s: command is required", e.Name())
		}
		if a.Prompt == "" {
			a.Prompt = "arg"
		}
		if a.Parse.Text == "" {
			return nil, fmt.Errorf("adapter %s: parse.text is required", e.Name())
		}
		adapters = append(adapters, a)
	}
	sort.Slice(adapters, func(i, j int) bool { return adapters[i].Name < adapters[j].Name })
	return adapters, nil
}
