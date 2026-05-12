package validation_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/validation"
)

func TestCheckTools_AllBuiltIn(t *testing.T) {
	t.Parallel()
	errs := validation.CheckTools([]string{"bash", "read_file", "write_file"}, "")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestCheckTools_CITools(t *testing.T) {
	t.Parallel()
	errs := validation.CheckTools([]string{"post_pr_comment", "create_pull_request"}, "")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestCheckTools_UnknownTool(t *testing.T) {
	t.Parallel()
	errs := validation.CheckTools([]string{"bash", "deploy"}, "")
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Tool != "deploy" {
		t.Errorf("tool = %q, want 'deploy'", errs[0].Tool)
	}
}

func TestCheckTools_CustomToolExists(t *testing.T) {
	t.Parallel()
	toolsDir := filepath.Join(t.TempDir(), "tools")
	os.MkdirAll(toolsDir, 0o755)
	os.WriteFile(filepath.Join(toolsDir, "deploy.yaml"), []byte("description: Deploy\nrun: echo deploy\n"), 0o644)

	errs := validation.CheckTools([]string{"deploy"}, toolsDir)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestCheckCustomTools_Valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("description: Deploy\nrun: echo deploy\n"), 0o644)

	errs := validation.CheckCustomTools(dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestCheckCustomTools_MissingDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("run: echo hi\n"), 0o644)

	errs := validation.CheckCustomTools(dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestCheckCustomTools_MissingRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("description: Does stuff\n"), 0o644)

	errs := validation.CheckCustomTools(dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestCheckCustomTools_InvalidParam(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("description: D\nrun: echo\nparameters:\n  x:\n    type: map\n"), 0o644)

	errs := validation.CheckCustomTools(dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestCheckCustomTools_NoDir(t *testing.T) {
	t.Parallel()
	errs := validation.CheckCustomTools(filepath.Join(t.TempDir(), "nonexistent"))
	if len(errs) != 0 {
		t.Errorf("missing dir should return no errors, got %v", errs)
	}
}
