// jerry init: scaffolds .jerry/ with a v3 workflow spec.

package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/errors"
)

//go:embed templates/review/workflow.yaml templates/review/reviewer.md
//go:embed templates/feature/workflow.yaml templates/feature/planner.md templates/feature/generator.md
//go:embed templates/settings.yaml
var embeddedTemplates embed.FS

func newInitCmd() *cobra.Command {
	var (
		targetPath string
		template   string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new .jerry/ directory",
		Long:  "Scaffolds a .jerry/ directory with a v3 workflow spec. Run `jerry generate` to compile CI config.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return Scaffold(targetPath, template)
		},
	}

	cmd.Flags().StringVar(&targetPath, "path", "", "Directory to initialize in (default: current directory)")
	cmd.Flags().StringVar(&template, "template", "", "Additional workflow template to add (e.g., feature)")

	return cmd
}

// Scaffold creates .jerry/ with the specified workflow template.
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
		if err := writeJerryGitignore(jerryDir); err != nil {
			return err
		}
		if err := ensureRootGitignore(targetPath); err != nil {
			return err
		}
	}

	printInitOutput(template)
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

// writeJerryGitignore ignores local-only files under .jerry/.
func writeJerryGitignore(jerryDir string) error {
	gitignorePath := filepath.Join(jerryDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		return nil
	}
	return os.WriteFile(gitignorePath, []byte("settings.local.yaml\n"), 0o644)
}

// ensureRootGitignore appends the ephemeral context directory to the repo's
// root .gitignore (it lives at the workspace root, not under .jerry/).
func ensureRootGitignore(targetPath string) error {
	const entry = ".jerry-run/"
	path := filepath.Join(targetPath, ".gitignore")
	existing, _ := os.ReadFile(path)
	if strings.Contains(string(existing), entry) {
		return nil
	}
	prefix := ""
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		prefix = "\n"
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return errors.Wrap(errors.CodeStateWriteFailed, "failed to update .gitignore", err)
	}
	defer f.Close()
	if _, err := f.WriteString(prefix + entry + "\n"); err != nil {
		return errors.Wrap(errors.CodeStateWriteFailed, "failed to write .gitignore", err)
	}
	return nil
}

func printInitOutput(template string) {
	if template == "review" {
		fmt.Println("Jerry initialized:")
	} else {
		fmt.Printf("Added %s workflow:\n", template)
	}

	entries, _ := embeddedTemplates.ReadDir("templates/" + template)
	for _, entry := range entries {
		fmt.Printf("  .jerry/%s/%s\n", template, entry.Name())
	}

	fmt.Println()
	fmt.Println("Validate:    jerry validate")
	fmt.Printf("Run locally: jerry run %s \"your task\"\n", template)
}
