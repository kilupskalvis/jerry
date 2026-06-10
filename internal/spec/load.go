package spec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

// Project is a fully loaded .jerry/ directory.
type Project struct {
	Root      string
	Workflows []*Workflow
	Settings  *Settings
	Lock      *Lockfile
}

// LoadProject loads every directory under root that contains a
// workflow.yaml, plus settings.yaml and jerry.lock. There is exactly one
// format; files that do not parse strictly are errors.
func LoadProject(root string) (*Project, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeJerryDirNotFound, root, err)
	}

	p := &Project{Root: root}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, statErr := os.Stat(filepath.Join(dir, "workflow.yaml")); statErr != nil {
			continue
		}
		wf, loadErr := LoadWorkflow(dir)
		if loadErr != nil {
			return nil, loadErr
		}
		p.Workflows = append(p.Workflows, wf)
	}
	sort.Slice(p.Workflows, func(i, j int) bool { return p.Workflows[i].Name < p.Workflows[j].Name })

	if p.Settings, err = LoadSettings(root); err != nil {
		return nil, err
	}
	if p.Lock, err = LoadLock(root); err != nil {
		return nil, err
	}
	return p, nil
}

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
