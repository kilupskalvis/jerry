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

//go:embed agents/context.md agents/plan.md agents/generate.md agents/feature.yaml agents/github-pr.sh agents/github-workflow.yml agents/gitlab-ci.yml
var embeddedAgents embed.FS

func newInitCmd() *cobra.Command {
	var (
		targetPath string
		ciPlatform string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new .motif/ directory",
		Long:  "Scaffolds a .motif/ directory with an example pipeline, core agents, scripts, and configuration.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runInit(targetPath, ciPlatform)
		},
	}

	cmd.Flags().StringVar(&targetPath, "path", "", "Directory to initialize in (default: current directory)")
	cmd.Flags().StringVar(&ciPlatform, "ci", "", "Generate CI workflow file (github or gitlab)")

	return cmd
}

func runInit(targetPath, ciPlatform string) error {
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

	// Write embedded agent definitions, feature pipeline, and publish script.
	embeddedFiles := map[string]struct {
		src  string
		perm os.FileMode
	}{
		filepath.Join(motifDir, "agents", "context.md"):      {src: "agents/context.md", perm: 0o644},
		filepath.Join(motifDir, "agents", "plan.md"):         {src: "agents/plan.md", perm: 0o644},
		filepath.Join(motifDir, "agents", "generate.md"):     {src: "agents/generate.md", perm: 0o644},
		filepath.Join(motifDir, "pipelines", "feature.yaml"): {src: "agents/feature.yaml", perm: 0o644},
		filepath.Join(motifDir, "scripts", "github-pr.sh"):   {src: "agents/github-pr.sh", perm: 0o755},
	}
	for destPath, meta := range embeddedFiles {
		content, readErr := embeddedAgents.ReadFile(meta.src)
		if readErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to read embedded file %q", meta.src), readErr)
		}
		if writeErr := os.WriteFile(destPath, content, meta.perm); writeErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to write %q", destPath), writeErr)
		}
	}

	// Generate CI workflow if requested.
	if ciPlatform != "" {
		if ciErr := generateCIConfig(targetPath, ciPlatform); ciErr != nil {
			return ciErr
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

func generateCIConfig(targetPath, platform string) error {
	switch platform {
	case "github":
		ciDir := filepath.Join(targetPath, ".github", "workflows")
		if mkdirErr := os.MkdirAll(ciDir, 0o755); mkdirErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed, "failed to create .github/workflows", mkdirErr)
		}
		content, readErr := embeddedAgents.ReadFile("agents/github-workflow.yml")
		if readErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed, "failed to read GitHub workflow template", readErr)
		}
		destPath := filepath.Join(ciDir, "motif.yml")
		if writeErr := os.WriteFile(destPath, content, 0o644); writeErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed, "failed to write GitHub workflow", writeErr)
		}
		fmt.Printf("  CI workflow: %s\n", destPath)

	case "gitlab":
		content, readErr := embeddedAgents.ReadFile("agents/gitlab-ci.yml")
		if readErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed, "failed to read GitLab CI template", readErr)
		}
		destPath := filepath.Join(targetPath, ".motif-gitlab-ci.yml")
		if writeErr := os.WriteFile(destPath, content, 0o644); writeErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed, "failed to write GitLab CI config", writeErr)
		}
		fmt.Printf("  CI config: %s (add contents to your .gitlab-ci.yml)\n", destPath)

	default:
		return fmt.Errorf("unknown CI platform %q (supported: github, gitlab)", platform)
	}
	return nil
}

const motifGitignore = `runs/
cache/
`
