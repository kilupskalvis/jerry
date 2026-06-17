package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/cli"
	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/spec"
)

func TestLockWritesPinnedVersion(t *testing.T) {
	repo := t.TempDir()
	jerryDir := filepath.Join(repo, ".jerry")
	if err := os.MkdirAll(jerryDir, 0o755); err != nil {
		t.Fatal(err)
	}

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "pi"),
		[]byte("#!/bin/sh\necho 0.73.1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	app := &cli.App{JerryDir: jerryDir, Printer: output.NewPrinter(os.Stderr, os.Stderr)}
	if err := cli.RunLock(app); err != nil {
		t.Fatalf("RunLock: %v", err)
	}

	lock, err := spec.LoadLock(jerryDir)
	if err != nil || lock == nil {
		t.Fatalf("LoadLock: %v", err)
	}
	if lock.Runtimes["pi"].Version != "0.73.1" {
		t.Errorf("pin = %q, want 0.73.1", lock.Runtimes["pi"].Version)
	}
}
