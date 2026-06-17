package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/runtime"
)

func v3ProjectWithLock(t *testing.T) (string, string) {
	t.Helper()
	repo, jerryDir := v3Project(t)
	lock := "version: 1\nruntimes:\n    pi:\n        package: \"@mariozechner/pi-coding-agent\"\n        version: 0.73.1\n"
	if err := os.WriteFile(filepath.Join(jerryDir, "jerry.lock"), []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, jerryDir
}

func TestGenerateWritesFiles(t *testing.T) {
	repo, jerryDir := v3ProjectWithLock(t)
	app := &App{
		JerryDir: jerryDir, RepoRoot: repo, Version: "0.1.0",
		Registry: runtime.NewRegistry(runtime.NewFake("pi")),
		Printer:  output.NewPrinter(os.Stderr, os.Stderr),
	}
	if err := runGenerate(app, false, false); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}

	path := filepath.Join(repo, ".github", "workflows", "jerry-demo.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("generated file missing: %v", err)
	}
	if len(data) == 0 {
		t.Error("generated file empty")
	}
}

func TestGenerateCheckPassesWhenFresh(t *testing.T) {
	repo, jerryDir := v3ProjectWithLock(t)
	app := &App{
		JerryDir: jerryDir, RepoRoot: repo, Version: "0.1.0",
		Registry: runtime.NewRegistry(runtime.NewFake("pi")),
		Printer:  output.NewPrinter(os.Stderr, os.Stderr),
	}
	if err := runGenerate(app, false, false); err != nil {
		t.Fatal(err)
	}
	if err := runGenerate(app, true, false); err != nil {
		t.Fatalf("--check failed on fresh output: %v", err)
	}
}

func TestGenerateCheckDetectsDrift(t *testing.T) {
	repo, jerryDir := v3ProjectWithLock(t)
	app := &App{
		JerryDir: jerryDir, RepoRoot: repo, Version: "0.1.0",
		Registry: runtime.NewRegistry(runtime.NewFake("pi")),
		Printer:  output.NewPrinter(os.Stderr, os.Stderr),
	}
	if err := runGenerate(app, false, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, ".github", "workflows", "jerry-demo.yml")
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runGenerate(app, true, false)
	if err == nil {
		t.Fatal("--check should detect drift")
	}
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) || ec.ExitCode() != 2 {
		t.Errorf("want exit 2, got %v", err)
	}
}

func TestGenerateCheckDetectsMissingFile(t *testing.T) {
	repo, jerryDir := v3ProjectWithLock(t)
	app := &App{
		JerryDir: jerryDir, RepoRoot: repo, Version: "0.1.0",
		Registry: runtime.NewRegistry(runtime.NewFake("pi")),
		Printer:  output.NewPrinter(os.Stderr, os.Stderr),
	}
	err := runGenerate(app, true, false)
	if err == nil {
		t.Fatal("--check should detect missing file")
	}
}
