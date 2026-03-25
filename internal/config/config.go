// Package config provides runtime configuration for Jerry.
// Config is loaded once at CLI startup and passed down via dependency injection.
// No package reads environment variables or files directly — all external
// input flows through the Config struct.
package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/kilupskalvis/jerry/internal/errors"
)

// DefaultStepTimeoutValue is the fallback timeout for steps that don't
// specify their own. Prevents runaway scripts from hanging forever.
const DefaultStepTimeoutValue = 10 * time.Minute

// Config holds all runtime configuration for a Jerry execution.
type Config struct {
	// JerryDir is the absolute path to the .jerry/ directory.
	JerryDir string

	// RepoRoot is the absolute path to the repository root
	// (parent of JerryDir).
	RepoRoot string

	// Env holds environment variables to pass to script steps.
	// Populated from the process environment at startup.
	// Scripts receive only PATH, HOME, JERRY_* vars, and
	// entries from this map.
	Env map[string]string

	// DefaultStepTimeout is the fallback timeout for steps that
	// don't specify their own.
	DefaultStepTimeout time.Duration

	// DefaultModel is the fallback model when an agent doesn't specify one.
	// Empty string means no default — agents must specify their own model.
	// Set from JERRY_DEFAULT_MODEL environment variable.
	DefaultModel string

	// FileConfig holds values loaded from .jerry/config.yaml.
	// nil if no config file was found.
	FileConfig *FileConfig
}

// FindJerryDir walks up from startDir looking for a .jerry/ directory.
// Returns the absolute path to .jerry/ and the repo root (its parent),
// or an error if not found before reaching the filesystem root.
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
			// Reached filesystem root without finding .jerry/
			return "", "", errors.New(errors.CodeJerryDirNotFound,
				"no .jerry/ directory found (searched from "+startDir+" to filesystem root)")
		}
		current = parent
	}
}
