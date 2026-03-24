// motif init: scaffolds a .motif/ directory with example pipeline and scripts.

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/motif/internal/errors"
)

func newInitCmd() *cobra.Command {
	var targetPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new .motif/ directory",
		Long:  "Scaffolds a .motif/ directory with an example pipeline, scripts, and configuration.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runInit(targetPath)
		},
	}

	cmd.Flags().StringVar(&targetPath, "path", "", "Directory to initialize in (default: current directory)")

	return cmd
}

func runInit(targetPath string) error {
	if targetPath == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return errors.Wrap(errors.CodeMotifDirNotFound, "failed to get current directory", cwdErr)
		}
		targetPath = cwd
	}

	motifDir := filepath.Join(targetPath, ".motif")

	// Check if already initialized
	if info, statErr := os.Stat(motifDir); statErr == nil && info.IsDir() {
		return errors.New(errors.CodeMotifDirExists,
			fmt.Sprintf("Motif is already initialized in %s", targetPath))
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(motifDir, "pipelines"),
		filepath.Join(motifDir, "agents"),
		filepath.Join(motifDir, "scripts"),
		filepath.Join(motifDir, "runs"),
		filepath.Join(motifDir, "cache"),
	}

	for _, dir := range dirs {
		if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to create directory %q", dir), mkdirErr)
		}
	}

	// Write template files
	files := map[string]string{
		filepath.Join(motifDir, "pipelines", "example.yaml"):  examplePipelineYAML,
		filepath.Join(motifDir, "scripts", "echo-context.sh"): echoContextScript,
		filepath.Join(motifDir, ".gitignore"):                 motifGitignore,
		filepath.Join(motifDir, "agents", ".gitkeep"):         "",
	}

	for path, content := range files {
		perm := os.FileMode(0o644)
		if filepath.Ext(path) == ".sh" {
			perm = 0o755
		}
		if writeErr := os.WriteFile(path, []byte(content), perm); writeErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to write %q", path), writeErr)
		}
	}

	fmt.Printf("Motif initialized in %s\n", targetPath)
	fmt.Println("Run 'motif run example' to try the example pipeline.")
	return nil
}

const examplePipelineYAML = `name: example
description: "Example Motif pipeline — run with 'motif run example'"

steps:
  - name: show-trigger
    script: |
      echo "Pipeline triggered. Context:"
      cat "$MOTIF_CONTEXT_FILE"

  - name: list-files
    script: ls -la

  - name: check-status
    script: |
      echo '{"status": "ok", "message": "example pipeline completed"}'
    output_key: result
`

const echoContextScript = `#!/bin/sh
# Example script that reads the Motif context.
# Usage: reference in a pipeline step as:
#   script: .motif/scripts/echo-context.sh

echo "=== Motif Context ==="
cat "$MOTIF_CONTEXT_FILE"
echo ""
echo "Run ID:    $MOTIF_RUN_ID"
echo "Step Name: $MOTIF_STEP_NAME"
`

const motifGitignore = `runs/
cache/
`
