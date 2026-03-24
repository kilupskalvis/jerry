// Package testutil provides test helpers for creating temporary Motif
// directory structures and test fixtures.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// SetupTestMotifDir creates a temporary directory with a .motif/ structure
// containing the given pipelines. Pipeline names are keys, YAML content is values.
// Uses t.TempDir() for automatic cleanup.
func SetupTestMotifDir(t *testing.T, pipelines map[string]string) string {
	t.Helper()

	tmpDir := t.TempDir()
	motifDir := filepath.Join(tmpDir, ".motif")
	pipelinesDir := filepath.Join(motifDir, "pipelines")
	agentsDir := filepath.Join(motifDir, "agents")
	scriptsDir := filepath.Join(motifDir, "scripts")
	runsDir := filepath.Join(motifDir, "runs")

	for _, dir := range []string{pipelinesDir, agentsDir, scriptsDir, runsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create directory %q: %v", dir, err)
		}
	}

	for name, content := range pipelines {
		WritePipeline(t, motifDir, name, content)
	}

	return tmpDir
}

// WritePipeline writes a pipeline YAML file to the given .motif/pipelines/ directory.
func WritePipeline(t *testing.T, motifDir, name, content string) {
	t.Helper()

	pipelinesDir := filepath.Join(motifDir, "pipelines")
	if err := os.MkdirAll(pipelinesDir, 0o755); err != nil {
		t.Fatalf("failed to create pipelines dir: %v", err)
	}

	path := filepath.Join(pipelinesDir, name+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write pipeline %q: %v", name, err)
	}
}
