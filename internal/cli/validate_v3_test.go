package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/output"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateV3Project(t *testing.T) {
	root := t.TempDir()
	jerryDir := filepath.Join(root, ".jerry")
	writeFile(t, filepath.Join(jerryDir, "review", "workflow.yaml"), `
version: 1
on:
  pull_request:
    types: [opened]
steps:
  - name: review
    prompt: reviewer.md
`)
	writeFile(t, filepath.Join(jerryDir, "review", "reviewer.md"), "Review it.\n")

	app := &App{JerryDir: jerryDir, Printer: output.NewPrinter(io.Discard, io.Discard)}
	if err := validateV3(app); err != nil {
		t.Fatalf("validateV3 on clean project: %v", err)
	}

	writeFile(t, filepath.Join(jerryDir, "review", "reviewer.md"),
		"Review ${{ steps.ghost.output }}\n")
	if err := validateV3(app); err == nil {
		t.Fatal("want validation failure for unknown step ref")
	}
}
