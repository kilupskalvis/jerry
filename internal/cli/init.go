// jerry init: scaffolds .jerry/ with workflow templates and CI config.

package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/errors"
)

//go:embed templates/review/workflow.yaml templates/review/reviewer.md
//go:embed templates/feature/workflow.yaml templates/feature/planner.md templates/feature/generator.md
//go:embed templates/ci/github-review.yml templates/ci/github-feature.yml
//go:embed templates/ci/gitlab-review.yml templates/ci/gitlab-feature.yml
//go:embed templates/settings.yaml
var embeddedTemplates embed.FS

func newInitCmd() *cobra.Command {
	var (
		targetPath string
		template   string
		ciPlatform string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new .jerry/ directory",
		Long:  "Scaffolds a .jerry/ directory with workflow templates and CI config.",
		RunE: func(_ *cobra.Command, _ []string) error {
			if ciPlatform != "" {
				return initCI(targetPath, ciPlatform)
			}
			return Scaffold(targetPath, template)
		},
	}

	cmd.Flags().StringVar(&targetPath, "path", "", "Directory to initialize in (default: current directory)")
	cmd.Flags().StringVar(&template, "template", "", "Additional workflow template to add (e.g., feature)")
	cmd.Flags().StringVar(&ciPlatform, "ci", "", "Generate CI config only (github or gitlab)")

	return cmd
}

// Scaffold creates .jerry/ with the specified workflow template and auto-detected CI config.
// @lattice:flow init
func Scaffold(targetPath, template string) error {
	if targetPath == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return errors.Wrap(errors.CodeJerryDirNotFound, "failed to get current directory", cwdErr)
		}
		targetPath = cwd
	}

	jerryDir := filepath.Join(targetPath, ".jerry")

	if template == "" {
		if info, statErr := os.Stat(jerryDir); statErr == nil && info.IsDir() {
			return errors.New(errors.CodeJerryDirExists,
				fmt.Sprintf("Jerry is already initialized in %s — use --template to add a workflow", targetPath))
		}
		template = "review"
	}

	if err := scaffoldTemplate(jerryDir, template); err != nil {
		return err
	}

	if template == "review" {
		if err := writeSettingsFile(jerryDir); err != nil {
			return err
		}
		if err := writeGitignore(jerryDir); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(jerryDir, "runs"), 0o755); err != nil {
			return errors.Wrap(errors.CodeStateWriteFailed, "failed to create runs directory", err)
		}
	}

	ciGenerated := autoDetectAndGenerateCI(targetPath, template)

	printInitOutput(template, jerryDir, ciGenerated)
	return nil
}

func scaffoldTemplate(jerryDir, template string) error {
	wfDir := filepath.Join(jerryDir, template)
	if info, err := os.Stat(wfDir); err == nil && info.IsDir() {
		return errors.New(errors.CodeJerryDirExists,
			fmt.Sprintf("workflow %q already exists in %s", template, jerryDir))
	}

	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		return errors.Wrap(errors.CodeStateWriteFailed,
			fmt.Sprintf("failed to create directory %q", wfDir), err)
	}

	templateDir := "templates/" + template
	entries, err := embeddedTemplates.ReadDir(templateDir)
	if err != nil {
		return fmt.Errorf("unknown template %q (available: review, feature)", template)
	}

	for _, entry := range entries {
		content, readErr := embeddedTemplates.ReadFile(templateDir + "/" + entry.Name())
		if readErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to read embedded file %q", entry.Name()), readErr)
		}
		destPath := filepath.Join(wfDir, entry.Name())
		if writeErr := os.WriteFile(destPath, content, 0o644); writeErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to write %q", destPath), writeErr)
		}
	}

	return nil
}

func autoDetectAndGenerateCI(targetPath, template string) string {
	if _, err := os.Stat(filepath.Join(targetPath, ".github")); err == nil {
		if generateCITemplate(targetPath, "github", template) == nil {
			return "github"
		}
	}
	if _, err := os.Stat(filepath.Join(targetPath, ".gitlab-ci.yml")); err == nil {
		if generateCITemplate(targetPath, "gitlab", template) == nil {
			return "gitlab"
		}
	}
	return ""
}

func initCI(targetPath, platform string) error {
	if targetPath == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return errors.Wrap(errors.CodeJerryDirNotFound, "failed to get current directory", cwdErr)
		}
		targetPath = cwd
	}

	jerryDir := filepath.Join(targetPath, ".jerry")
	entries, err := os.ReadDir(jerryDir)
	if err != nil {
		return errors.New(errors.CodeJerryDirNotFound,
			"no .jerry/ directory found — run 'jerry init' first")
	}

	generated := 0
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "runs" || entry.Name() == "tools" {
			continue
		}
		wfFile := filepath.Join(jerryDir, entry.Name(), "workflow.yaml")
		if _, err := os.Stat(wfFile); err != nil {
			continue
		}
		if generateCITemplate(targetPath, platform, entry.Name()) == nil {
			fmt.Fprintf(os.Stderr, "  Generated CI config for %s workflow (%s)\n", entry.Name(), platform)
			generated++
		}
	}

	if generated == 0 {
		return fmt.Errorf("no workflows found in .jerry/ to generate CI config for")
	}
	return nil
}

func generateCITemplate(targetPath, platform, template string) error {
	ciFile := fmt.Sprintf("templates/ci/%s-%s.yml", platform, template)
	content, err := embeddedTemplates.ReadFile(ciFile)
	if err != nil {
		return err
	}

	switch platform {
	case "github":
		destDir := filepath.Join(targetPath, ".github", "workflows")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		dest := filepath.Join(destDir, fmt.Sprintf("jerry-%s.yml", template))
		if _, err := os.Stat(dest); err == nil {
			return nil
		}
		return os.WriteFile(dest, content, 0o644)
	case "gitlab":
		dest := filepath.Join(targetPath, fmt.Sprintf(".jerry-%s-ci.yml", template))
		if _, err := os.Stat(dest); err == nil {
			return nil
		}
		return os.WriteFile(dest, content, 0o644)
	}
	return fmt.Errorf("unknown CI platform %q (use github or gitlab)", platform)
}

func writeSettingsFile(jerryDir string) error {
	settingsPath := filepath.Join(jerryDir, "settings.yaml")
	if _, err := os.Stat(settingsPath); err == nil {
		return nil
	}
	content, err := embeddedTemplates.ReadFile("templates/settings.yaml")
	if err != nil {
		return errors.Wrap(errors.CodeStateWriteFailed, "failed to read settings template", err)
	}
	return os.WriteFile(settingsPath, content, 0o644)
}

func writeGitignore(jerryDir string) error {
	gitignorePath := filepath.Join(jerryDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		return nil
	}
	return os.WriteFile(gitignorePath, []byte("runs/\nsettings.local.yaml\n"), 0o644)
}

func printInitOutput(template, jerryDir, ciPlatform string) {
	if template == "review" {
		fmt.Println("Jerry initialized:")
	} else {
		fmt.Printf("Added %s workflow:\n", template)
	}

	entries, _ := embeddedTemplates.ReadDir("templates/" + template)
	for _, entry := range entries {
		fmt.Printf("  .jerry/%s/%s\n", template, entry.Name())
	}

	switch ciPlatform {
	case "github":
		fmt.Printf("  .github/workflows/jerry-%s.yml\n", template)
	case "gitlab":
		fmt.Printf("  .jerry-%s-ci.yml\n", template)
	}

	fmt.Println()
	if ciPlatform == "" {
		fmt.Println("No CI platform detected. Generate CI config with:")
		fmt.Println("  jerry init --ci github")
		fmt.Println("  jerry init --ci gitlab")
		fmt.Println()
	}

	fmt.Printf("Run locally: jerry run %s \"your task description\"\n", template)
}
