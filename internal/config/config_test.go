package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kilupskalvis/motif/internal/config"
)

func TestFindMotifDir_Found(t *testing.T) {
	tmpDir := t.TempDir()
	motifDir := filepath.Join(tmpDir, ".motif")
	if err := os.Mkdir(motifDir, 0755); err != nil {
		t.Fatalf("failed to create .motif dir: %v", err)
	}

	foundMotifDir, foundRepoRoot, err := config.FindMotifDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if foundMotifDir != motifDir {
		t.Errorf("motifDir = %q, want %q", foundMotifDir, motifDir)
	}
	if foundRepoRoot != tmpDir {
		t.Errorf("repoRoot = %q, want %q", foundRepoRoot, tmpDir)
	}
}

func TestFindMotifDir_FoundInParent(t *testing.T) {
	tmpDir := t.TempDir()
	motifDir := filepath.Join(tmpDir, ".motif")
	if err := os.Mkdir(motifDir, 0755); err != nil {
		t.Fatalf("failed to create .motif dir: %v", err)
	}

	// Create nested subdirectories
	nested := filepath.Join(tmpDir, "src", "handlers")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	foundMotifDir, foundRepoRoot, err := config.FindMotifDir(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if foundMotifDir != motifDir {
		t.Errorf("motifDir = %q, want %q", foundMotifDir, motifDir)
	}
	if foundRepoRoot != tmpDir {
		t.Errorf("repoRoot = %q, want %q", foundRepoRoot, tmpDir)
	}
}

func TestFindMotifDir_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, err := config.FindMotifDir(tmpDir)
	if err == nil {
		t.Fatal("expected error when .motif/ does not exist")
	}
}

func TestFindMotifDir_ReturnsAbsolutePaths(t *testing.T) {
	tmpDir := t.TempDir()
	motifDir := filepath.Join(tmpDir, ".motif")
	if err := os.Mkdir(motifDir, 0755); err != nil {
		t.Fatalf("failed to create .motif dir: %v", err)
	}

	foundMotifDir, foundRepoRoot, err := config.FindMotifDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(foundMotifDir) {
		t.Errorf("motifDir %q is not absolute", foundMotifDir)
	}
	if !filepath.IsAbs(foundRepoRoot) {
		t.Errorf("repoRoot %q is not absolute", foundRepoRoot)
	}
}

func TestFindMotifDir_IgnoresFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Create .motif as a file, not a directory
	motifFile := filepath.Join(tmpDir, ".motif")
	if err := os.WriteFile(motifFile, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("failed to create .motif file: %v", err)
	}

	_, _, err := config.FindMotifDir(tmpDir)
	if err == nil {
		t.Fatal("expected error when .motif is a file, not a directory")
	}
}

func TestDefaultStepTimeoutValue(t *testing.T) {
	expected := 10 * time.Minute
	if config.DefaultStepTimeoutValue != expected {
		t.Errorf("DefaultStepTimeoutValue = %v, want %v", config.DefaultStepTimeoutValue, expected)
	}
}
