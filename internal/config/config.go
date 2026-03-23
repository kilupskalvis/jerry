// Package config provides runtime configuration for Motif.
// Config is loaded once at CLI startup and passed down via dependency injection.
// No package reads environment variables or files directly — all external
// input flows through the Config struct.
package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/kilupskalvis/motif/internal/errors"
)

// DefaultStepTimeoutValue is the fallback timeout for steps that don't
// specify their own. Prevents runaway scripts from hanging forever.
const DefaultStepTimeoutValue = 10 * time.Minute

// Config holds all runtime configuration for a Motif execution.
type Config struct {
	// MotifDir is the absolute path to the .motif/ directory.
	MotifDir string

	// RepoRoot is the absolute path to the repository root
	// (parent of MotifDir).
	RepoRoot string

	// Env holds environment variables to pass to script steps.
	// Populated from the process environment at startup.
	// Scripts receive only PATH, HOME, MOTIF_* vars, and
	// entries from this map.
	Env map[string]string

	// DefaultStepTimeout is the fallback timeout for steps that
	// don't specify their own.
	DefaultStepTimeout time.Duration
}

// FindMotifDir walks up from startDir looking for a .motif/ directory.
// Returns the absolute path to .motif/ and the repo root (its parent),
// or an error if not found before reaching the filesystem root.
func FindMotifDir(startDir string) (motifDir string, repoRoot string, err error) {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", errors.Wrap(errors.CodeMotifDirNotFound,
			"failed to resolve absolute path", err)
	}

	for {
		candidate := filepath.Join(current, ".motif")
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.IsDir() {
			return candidate, current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding .motif/
			return "", "", errors.New(errors.CodeMotifDirNotFound,
				"no .motif/ directory found (searched from "+startDir+" to filesystem root)")
		}
		current = parent
	}
}
