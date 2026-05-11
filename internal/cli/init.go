// jerry init: scaffolds a .jerry/ directory with an example review workflow.

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
var embeddedTemplates embed.FS

func newInitCmd() *cobra.Command {
	var targetPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new .jerry/ directory",
		Long:  "Scaffolds a .jerry/ directory with an example review workflow.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return Scaffold(targetPath)
		},
	}

	cmd.Flags().StringVar(&targetPath, "path", "", "Directory to initialize in (default: current directory)")

	return cmd
}

// Scaffold creates a .jerry/ directory with an example review workflow.
// @lattice:flow init
func Scaffold(targetPath string) error {
	if targetPath == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return errors.Wrap(errors.CodeJerryDirNotFound, "failed to get current directory", cwdErr)
		}
		targetPath = cwd
	}

	jerryDir := filepath.Join(targetPath, ".jerry")

	if info, statErr := os.Stat(jerryDir); statErr == nil && info.IsDir() {
		return errors.New(errors.CodeJerryDirExists,
			fmt.Sprintf("Jerry is already initialized in %s", targetPath))
	}

	dirs := []string{
		filepath.Join(jerryDir, "review"),
		filepath.Join(jerryDir, "runs"),
	}
	for _, dir := range dirs {
		if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to create directory %q", dir), mkdirErr)
		}
	}

	if writeErr := os.WriteFile(filepath.Join(jerryDir, ".gitignore"), []byte(jerryGitignore), 0o644); writeErr != nil {
		return errors.Wrap(errors.CodeStateWriteFailed, "failed to write .gitignore", writeErr)
	}

	templateFiles := map[string]string{
		filepath.Join(jerryDir, "review", "workflow.yaml"): "templates/review/workflow.yaml",
		filepath.Join(jerryDir, "review", "reviewer.md"):   "templates/review/reviewer.md",
	}
	for destPath, srcPath := range templateFiles {
		content, readErr := embeddedTemplates.ReadFile(srcPath)
		if readErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to read embedded file %q", srcPath), readErr)
		}
		if writeErr := os.WriteFile(destPath, content, 0o644); writeErr != nil {
			return errors.Wrap(errors.CodeStateWriteFailed,
				fmt.Sprintf("failed to write %q", destPath), writeErr)
		}
	}

	fmt.Printf("Jerry initialized in %s\n", targetPath)
	fmt.Println("  Workflow: review/")
	fmt.Println("    workflow.yaml")
	fmt.Println("    reviewer.md")
	fmt.Println("")
	fmt.Println("Run 'jerry run review \"Check for common issues\"' to try it.")
	return nil
}

const jerryGitignore = `runs/
`
