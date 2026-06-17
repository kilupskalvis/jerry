// jerry generate: compile .jerry/ specs into native CI config files.

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/jerry/internal/compile"
	"github.com/kilupskalvis/jerry/internal/compile/github"
	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/spec"
)

// @lattice:flow generate
func newGenerateCmd(app *App) *cobra.Command {
	var (
		check  bool
		stdout bool
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Compile .jerry/ specs into CI config",
		Long:  "Generates native CI workflow files from .jerry/ specs. Use --check to verify generated files are up to date.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if app.JerryDir == "" {
				return jerrerr.New(jerrerr.CodeJerryDirNotFound,
					"not in a Jerry project (no .jerry/ directory found)")
			}
			return runGenerate(app, check, stdout)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Verify generated files match disk (exit 2 on drift)")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "Print generated files to stdout instead of writing")
	return cmd
}

func runGenerate(app *App, check, stdout bool) error {
	project, err := spec.LoadProject(app.JerryDir)
	if err != nil {
		return err
	}
	if issues := spec.ValidateProject(project); spec.HasErrors(issues) {
		return fmt.Errorf("spec validation failed; run `jerry validate`")
	}

	plan, err := compile.PlanProject(project, app.Version)
	if err != nil {
		return err
	}
	files, err := github.Emit(plan)
	if err != nil {
		return err
	}

	if check {
		return checkDrift(app, files)
	}
	if stdout {
		return printFiles(files)
	}
	return writeFiles(app, files)
}

func checkDrift(app *App, files []compile.GeneratedFile) error {
	var drifted []string
	for _, f := range files {
		diskPath := filepath.Join(app.RepoRoot, f.Path)
		existing, err := os.ReadFile(diskPath)
		if err != nil {
			drifted = append(drifted, f.Path+" (missing)")
			continue
		}
		if !bytes.Equal(existing, f.Content) {
			drifted = append(drifted, f.Path+" (changed)")
		}
	}
	if len(drifted) > 0 {
		for _, d := range drifted {
			app.Printer.Warning("drift: %s", d)
		}
		return execExit{code: 2}
	}
	app.Printer.Info("all generated files up to date")
	return nil
}

func printFiles(files []compile.GeneratedFile) error {
	for _, f := range files {
		fmt.Printf("--- %s ---\n%s", f.Path, f.Content)
	}
	return nil
}

func writeFiles(app *App, files []compile.GeneratedFile) error {
	for _, f := range files {
		path := filepath.Join(app.RepoRoot, f.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", f.Path, err)
		}
		if err := os.WriteFile(path, f.Content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", f.Path, err)
		}
		app.Printer.Info("wrote %s", f.Path)
	}
	return nil
}
