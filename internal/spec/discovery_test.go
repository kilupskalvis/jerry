package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindJerryDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".jerry"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	jerryDir, repoRoot, err := FindJerryDir(nested)
	if err != nil {
		t.Fatalf("FindJerryDir: %v", err)
	}
	if !filepath.IsAbs(jerryDir) || !filepath.IsAbs(repoRoot) {
		t.Errorf("paths should be absolute: %q %q", jerryDir, repoRoot)
	}
	if filepath.Base(jerryDir) != ".jerry" {
		t.Errorf("jerryDir = %q", jerryDir)
	}
}

func TestFindJerryDirNotFound(t *testing.T) {
	if _, _, err := FindJerryDir(t.TempDir()); err == nil {
		t.Fatal("want error when no .jerry/ exists")
	}
}

func TestFindJerryDirIgnoresFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".jerry"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := FindJerryDir(root); err == nil {
		t.Fatal("a .jerry file (not dir) must not satisfy the search")
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\nexport FOO=bar\nQUOTED=\"hello world\"\nSINGLE='x'\nblank line ignored\n\nNOEQ\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	env, err := LoadDotEnv(dir, ".env")
	if err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if env["FOO"] != "bar" || env["QUOTED"] != "hello world" || env["SINGLE"] != "x" {
		t.Errorf("env = %v", env)
	}
	if _, ok := env["NOEQ"]; ok {
		t.Error("lines without = should be skipped")
	}
}

func TestLoadDotEnvAbsent(t *testing.T) {
	env, err := LoadDotEnv(t.TempDir(), ".env")
	if err != nil {
		t.Fatalf("absent .env should not error: %v", err)
	}
	if len(env) != 0 {
		t.Errorf("want empty map, got %v", env)
	}
}
