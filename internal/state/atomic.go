package state

import (
	"encoding/json"
	"os"

	"github.com/kilupskalvis/motif/internal/errors"
)

// AtomicWriteJSON writes data as formatted JSON to the given path atomically.
// It writes to a temporary file first, syncs it, then renames — preventing
// corruption if the process crashes mid-write.
func AtomicWriteJSON(path string, data any) error {
	content, marshalErr := json.MarshalIndent(data, "", "  ")
	if marshalErr != nil {
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to marshal JSON", marshalErr)
	}

	tmpPath := path + ".tmp"
	tmpFile, createErr := os.Create(tmpPath)
	if createErr != nil {
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to create temp file", createErr)
	}

	if _, writeErr := tmpFile.Write(content); writeErr != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to write temp file", writeErr)
	}

	// Ensure data is flushed to disk before rename
	if syncErr := tmpFile.Sync(); syncErr != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to sync temp file", syncErr)
	}

	if closeErr := tmpFile.Close(); closeErr != nil {
		os.Remove(tmpPath)
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to close temp file", closeErr)
	}

	// Atomic rename (on POSIX systems)
	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		os.Remove(tmpPath)
		return errors.Wrap(errors.CodeStateWriteFailed,
			"failed to rename temp file to target", renameErr)
	}

	return nil
}
