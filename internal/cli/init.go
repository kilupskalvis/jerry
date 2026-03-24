// motif init: scaffolds a .motif/ directory with example pipeline, core agents, and scripts.

package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/motif/internal/errors"
)

//go:embed agents/context.md agents/plan.md agents/generate.md agents/feature.yaml
var embeddedAgents embed.FS

func newInitCmd() *cobra.Command {
	var targetPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new .motif/ directory",
		Long:  "Scaffolds a .motif/ directory with an example pipeline, core agents, scripts, and configuration.",
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

	if info, statErr := os.Stat(motifDir); statErr == nil && info.IsDir() {
		return errors.New(errors.CodeMotifDirExists,
			fmt.Sprintf("Motif is already initialized in %s", targetPath))
	}

	// Create directory structure.
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

	// Write static template files.
	staticFiles := map[string]string{
		filepath.Join(motifDir, "pipelines", "example.yaml"):  examplePipelineYAML,
		filepath.Join(motifDir, "scripts", "echo-context.sh"): echoContextScript,
		filepath.Join(motifDir, ".gitignore"):                 motifGitignore,
		filepath.Join(motifDir, "config.yaml"):                defaultConfigYAML,
	}
	for path, content := range staticFiles {
		perm := os.FileMode(0o644)
		if filepath.Ext(path) == ".sh" {
			perm = 0o755
		}
		if writeErr := os.WriteFile(path, []byte(content), perm); writeErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to write %q", path), writeErr)
		}
	}

	// Write embedded agent definitions and feature pipeline.
	embeddedFiles := map[string]string{
		filepath.Join(motifDir, "agents", "context.md"):      "agents/context.md",
		filepath.Join(motifDir, "agents", "plan.md"):         "agents/plan.md",
		filepath.Join(motifDir, "agents", "generate.md"):     "agents/generate.md",
		filepath.Join(motifDir, "pipelines", "feature.yaml"): "agents/feature.yaml",
	}
	for destPath, embedPath := range embeddedFiles {
		content, readErr := embeddedAgents.ReadFile(embedPath)
		if readErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to read embedded file %q", embedPath), readErr)
		}
		if writeErr := os.WriteFile(destPath, content, 0o644); writeErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to write %q", destPath), writeErr)
		}
	}

	fmt.Printf("Motif initialized in %s\n", targetPath)
	fmt.Println("  Core agents: context.md, plan.md, generate.md")
	fmt.Println("  Pipelines:   example.yaml, feature.yaml")
	fmt.Println("")
	fmt.Println("Run 'motif run example' to try the example pipeline.")
	fmt.Println("Run 'motif run feature --intent \"...\"' to generate code with AI agents.")
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

const defaultConfigYAML = `# Motif configuration — defaults for all agents and pipelines.
# Agent frontmatter values take precedence over these defaults.
#
# defaults:
#   model: claude-sonnet-4-6
#   timeout: 600s
#   max_iterations: 50
#   context_window: 200000
`

const motifGitignore = `runs/
cache/
`
