// Package config provides runtime configuration for Jerry.
package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/kilupskalvis/jerry/internal/errors"
)

// DefaultStepTimeoutValue is the fallback timeout for steps without an explicit timeout.
const DefaultStepTimeoutValue = 10 * time.Minute

// FindJerryDir walks up from startDir looking for a .jerry/ directory.
func FindJerryDir(startDir string) (jerryDir, repoRoot string, err error) {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", errors.Wrap(errors.CodeJerryDirNotFound,
			"failed to resolve absolute path", err)
	}

	for {
		candidate := filepath.Join(current, ".jerry")
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.IsDir() {
			return candidate, current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", "", errors.New(errors.CodeJerryDirNotFound,
				"no .jerry/ directory found (searched from "+startDir+" to filesystem root)")
		}
		current = parent
	}
}
