package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kilupskalvis/jerry/internal/config"
)

func TestFindJerryDir_Found(t *testing.T) {
	tmpDir := t.TempDir()
	jerryDir := filepath.Join(tmpDir, ".jerry")
	if err := os.Mkdir(jerryDir, 0o755); err != nil {
		t.Fatalf("failed to create .jerry dir: %v", err)
	}

	foundJerryDir, foundRepoRoot, err := config.FindJerryDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if foundJerryDir != jerryDir {
		t.Errorf("jerryDir = %q, want %q", foundJerryDir, jerryDir)
	}
	if foundRepoRoot != tmpDir {
		t.Errorf("repoRoot = %q, want %q", foundRepoRoot, tmpDir)
	}
}

func TestFindJerryDir_FoundInParent(t *testing.T) {
	tmpDir := t.TempDir()
	jerryDir := filepath.Join(tmpDir, ".jerry")
	if err := os.Mkdir(jerryDir, 0o755); err != nil {
		t.Fatalf("failed to create .jerry dir: %v", err)
	}

	// Create nested subdirectories
	nested := filepath.Join(tmpDir, "src", "handlers")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	foundJerryDir, foundRepoRoot, err := config.FindJerryDir(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if foundJerryDir != jerryDir {
		t.Errorf("jerryDir = %q, want %q", foundJerryDir, jerryDir)
	}
	if foundRepoRoot != tmpDir {
		t.Errorf("repoRoot = %q, want %q", foundRepoRoot, tmpDir)
	}
}

func TestFindJerryDir_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, err := config.FindJerryDir(tmpDir)
	if err == nil {
		t.Fatal("expected error when .jerry/ does not exist")
	}
}

func TestFindJerryDir_ReturnsAbsolutePaths(t *testing.T) {
	tmpDir := t.TempDir()
	jerryDir := filepath.Join(tmpDir, ".jerry")
	if err := os.Mkdir(jerryDir, 0o755); err != nil {
		t.Fatalf("failed to create .jerry dir: %v", err)
	}

	foundJerryDir, foundRepoRoot, err := config.FindJerryDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(foundJerryDir) {
		t.Errorf("jerryDir %q is not absolute", foundJerryDir)
	}
	if !filepath.IsAbs(foundRepoRoot) {
		t.Errorf("repoRoot %q is not absolute", foundRepoRoot)
	}
}

func TestFindJerryDir_IgnoresFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Create .jerry as a file, not a directory
	jerryFile := filepath.Join(tmpDir, ".jerry")
	if err := os.WriteFile(jerryFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create .jerry file: %v", err)
	}

	_, _, err := config.FindJerryDir(tmpDir)
	if err == nil {
		t.Fatal("expected error when .jerry is a file, not a directory")
	}
}

func TestDefaultStepTimeoutValue(t *testing.T) {
	expected := 10 * time.Minute
	if config.DefaultStepTimeoutValue != expected {
		t.Errorf("DefaultStepTimeoutValue = %v, want %v", config.DefaultStepTimeoutValue, expected)
	}
}
