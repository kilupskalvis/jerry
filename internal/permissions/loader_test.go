package permissions_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/permissions"
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

func TestLoadSettings_BasicDeny(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.yaml"), `
permissions:
  deny:
    - bash: ["rm *", "curl *"]
    - read_file: ["*.env"]
`)

	perms, err := permissions.LoadSettings(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bashDeny := perms.DenyFor("bash")
	if len(bashDeny) != 2 {
		t.Errorf("bash deny = %d patterns, want 2", len(bashDeny))
	}

	readDeny := perms.DenyFor("read_file")
	if len(readDeny) != 1 {
		t.Errorf("read_file deny = %d patterns, want 1", len(readDeny))
	}
}

func TestLoadSettings_WithAllow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.yaml"), `
permissions:
  allow:
    - bash: ["go *", "npm *"]
    - write_file: ["src/**"]
`)

	perms, err := permissions.LoadSettings(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bashAllow := perms.AllowFor("bash")
	if len(bashAllow) != 2 {
		t.Errorf("bash allow = %d patterns, want 2", len(bashAllow))
	}
}

func TestLoadSettings_LocalMerge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.yaml"), `
permissions:
  deny:
    - bash: ["rm *"]
`)
	writeFile(t, filepath.Join(dir, "settings.local.yaml"), `
permissions:
  deny:
    - bash: ["docker *"]
`)

	perms, err := permissions.LoadSettings(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bashDeny := perms.DenyFor("bash")
	if len(bashDeny) != 2 {
		t.Errorf("bash deny = %d patterns after local merge, want 2", len(bashDeny))
	}
}

func TestLoadSettings_NoFiles(t *testing.T) {
	t.Parallel()
	perms, err := permissions.LoadSettings(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(perms.Deny) != 0 || len(perms.Allow) != 0 {
		t.Error("empty dir should yield empty permissions")
	}
}

func TestLoadSettings_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.yaml"), `permissions: [invalid`)

	_, err := permissions.LoadSettings(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
