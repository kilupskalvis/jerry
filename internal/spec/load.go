package spec

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// parseWorkflow strictly decodes workflow.yaml bytes. Unknown fields are
// errors (typo safety); callers add did-you-mean context.
func parseWorkflow(data []byte) (*Workflow, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var wf Workflow
	if err := dec.Decode(&wf); err != nil {
		return nil, fmt.Errorf("parsing workflow.yaml: %w", err)
	}
	return &wf, nil
}
