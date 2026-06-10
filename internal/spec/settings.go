package spec

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

// Settings is the optional .jerry/settings.yaml org policy.
type Settings struct {
	Policy Policy `yaml:"policy"`
}

// Policy holds non-overridable org rules enforced at validate, generate,
// and exec time.
type Policy struct {
	Deny     []string      `yaml:"deny,omitempty"`
	Budget   PolicyBudget  `yaml:"budget,omitempty"`
	Runtimes RuntimePolicy `yaml:"runtimes,omitempty"`
}

type PolicyBudget struct {
	MaxCostPerRun float64 `yaml:"max_cost_per_run,omitempty"`
}

type RuntimePolicy struct {
	Allowed []string `yaml:"allowed,omitempty"`
}

// LoadSettings reads <root>/settings.yaml. A missing file returns
// (nil, nil): settings are optional.
func LoadSettings(root string) (*Settings, error) {
	data, err := os.ReadFile(filepath.Join(root, "settings.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeConfigInvalid, "reading settings.yaml", err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var s Settings
	if err := dec.Decode(&s); err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeConfigInvalid, "parsing settings.yaml", err)
	}
	return &s, nil
}
