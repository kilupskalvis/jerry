package spec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
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

// LoadWorkflow loads and parses <dir>/workflow.yaml. It does not validate;
// callers run ValidateWorkflow / ValidateProject on the result.
func LoadWorkflow(dir string) (*Workflow, error) {
	path := filepath.Join(dir, "workflow.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeWorkflowNotFound,
			fmt.Sprintf("reading %s", path), err)
	}

	wf, err := parseWorkflow(data)
	if err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeInvalidWorkflow, path, err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", dir, err)
	}
	wf.Dir = abs
	wf.Name = filepath.Base(abs)
	return wf, nil
}

// PromptText resolves a step's prompt: values ending in ".md" are file
// references relative to the workflow directory; anything else is inline.
func (w *Workflow) PromptText(s *Step) (string, error) {
	if !strings.HasSuffix(s.Prompt, ".md") || strings.ContainsAny(s.Prompt, "\n") {
		return s.Prompt, nil
	}
	path := filepath.Join(w.Dir, s.Prompt)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", jerrerr.Wrap(jerrerr.CodeInvalidWorkflow,
			fmt.Sprintf("step %q: prompt file %s", s.Name, s.Prompt), err)
	}
	return string(data), nil
}
