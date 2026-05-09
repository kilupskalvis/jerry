package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv_Missing(t *testing.T) {
	env, err := LoadDotEnv(t.TempDir(), ".env")
	if err != nil {
		t.Fatalf("missing .env should not error: %v", err)
	}
	if len(env) != 0 {
		t.Errorf("expected empty map for missing .env, got %d entries", len(env))
	}
}

func TestLoadDotEnv_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	content := "KEY1=value1\nKEY2=\"quoted value\"\nexport KEY3=exported\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := LoadDotEnv(tmpDir, ".env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["KEY1"] != "value1" {
		t.Errorf("KEY1 = %q, want %q", env["KEY1"], "value1")
	}
	if env["KEY2"] != "quoted value" {
		t.Errorf("KEY2 = %q, want %q", env["KEY2"], "quoted value")
	}
	if env["KEY3"] != "exported" {
		t.Errorf("KEY3 = %q, want %q", env["KEY3"], "exported")
	}
}
