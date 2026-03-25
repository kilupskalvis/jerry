// Package testutil provides test helpers for creating temporary Jerry
// directory structures and test fixtures.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// SetupTestJerryDir creates a temporary directory with a .jerry/ structure
// containing the given pipelines. Pipeline names are keys, YAML content is values.
// Uses t.TempDir() for automatic cleanup.
func SetupTestJerryDir(t *testing.T, pipelines map[string]string) string {
	t.Helper()

	tmpDir := t.TempDir()
	jerryDir := filepath.Join(tmpDir, ".jerry")
	pipelinesDir := filepath.Join(jerryDir, "pipelines")
	agentsDir := filepath.Join(jerryDir, "agents")
	scriptsDir := filepath.Join(jerryDir, "scripts")
	runsDir := filepath.Join(jerryDir, "runs")

	for _, dir := range []string{pipelinesDir, agentsDir, scriptsDir, runsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create directory %q: %v", dir, err)
		}
	}

	for name, content := range pipelines {
		WritePipeline(t, jerryDir, name, content)
	}

	return tmpDir
}

// WritePipeline writes a pipeline YAML file to the given .jerry/pipelines/ directory.
func WritePipeline(t *testing.T, jerryDir, name, content string) {
	t.Helper()

	pipelinesDir := filepath.Join(jerryDir, "pipelines")
	if err := os.MkdirAll(pipelinesDir, 0o755); err != nil {
		t.Fatalf("failed to create pipelines dir: %v", err)
	}

	path := filepath.Join(pipelinesDir, name+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write pipeline %q: %v", name, err)
	}
}
