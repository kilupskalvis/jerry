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

// Lockfile pins runtime versions (jerry.lock), terraform-lock style.
type Lockfile struct {
	Version  int                      `yaml:"version"`
	Runtimes map[string]LockedRuntime `yaml:"runtimes"`
}

// LockedRuntime is one pinned runtime entry.
type LockedRuntime struct {
	Package  string `yaml:"package"`
	Version  string `yaml:"version"`
	Checksum string `yaml:"checksum,omitempty"`
}

// LoadLock reads <root>/jerry.lock. A missing file returns (nil, nil):
// validation downgrades unpinned runtimes to a warning.
func LoadLock(root string) (*Lockfile, error) {
	data, err := os.ReadFile(filepath.Join(root, "jerry.lock"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeConfigInvalid, "reading jerry.lock", err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var l Lockfile
	if err := dec.Decode(&l); err != nil {
		return nil, jerrerr.Wrap(jerrerr.CodeConfigInvalid, "parsing jerry.lock", err)
	}
	return &l, nil
}
