package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFileConfig_Valid(t *testing.T) {
	cfg, err := LoadFileConfig("../../testdata/config", "valid-config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Defaults.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want %q", cfg.Defaults.Model, "claude-sonnet-4-6")
	}
	if cfg.Defaults.Timeout.Duration != 600*time.Second {
		t.Errorf("timeout = %v, want %v", cfg.Defaults.Timeout.Duration, 600*time.Second)
	}
	if cfg.Defaults.MaxIterations != 50 {
		t.Errorf("max_iterations = %d, want %d", cfg.Defaults.MaxIterations, 50)
	}
	if cfg.Defaults.ContextWindow != 200000 {
		t.Errorf("context_window = %d, want %d", cfg.Defaults.ContextWindow, 200000)
	}
}

func TestLoadFileConfig_ValidWithModels(t *testing.T) {
	cfg, err := LoadFileConfig("../../testdata/config", "valid-with-models.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Defaults.Model != "gpt-4o" {
		t.Errorf("model = %q, want %q", cfg.Defaults.Model, "gpt-4o")
	}
	if cfg.Defaults.Timeout.Duration != 300*time.Second {
		t.Errorf("timeout = %v, want %v", cfg.Defaults.Timeout.Duration, 300*time.Second)
	}
	// Unset fields should be zero-valued.
	if cfg.Defaults.MaxIterations != 0 {
		t.Errorf("max_iterations = %d, want 0 (unset)", cfg.Defaults.MaxIterations)
	}
}

func TestLoadFileConfig_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFileConfig(tmpDir, "config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Defaults.Model != "" {
		t.Errorf("model should be empty for empty file, got %q", cfg.Defaults.Model)
	}
}

func TestLoadFileConfig_Missing(t *testing.T) {
	cfg, err := LoadFileConfig(t.TempDir(), "config.yaml")
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.Defaults.Model != "" {
		t.Errorf("model should be empty for missing file, got %q", cfg.Defaults.Model)
	}
}

func TestLoadFileConfig_Invalid(t *testing.T) {
	_, err := LoadFileConfig("../../testdata/config", "invalid-config.yaml")
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

func TestLoadDotEnv_Valid(t *testing.T) {
	env, err := LoadDotEnv("../../testdata/config", "valid-env-file.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-test-key-123",
		"OPENAI_API_KEY":    "sk-test-key-456",
		"QUOTED_DOUBLE":     "hello world",
		"QUOTED_SINGLE":     "hello world",
		"EXPORTED_VAR":      "exported_value",
		"EMPTY_VALUE":       "",
		"EQUALS_IN_VALUE":   "key=value=more",
	}

	for key, wantVal := range want {
		got, ok := env[key]
		if !ok {
			t.Errorf("missing key %q", key)
			continue
		}
		if got != wantVal {
			t.Errorf("%s = %q, want %q", key, got, wantVal)
		}
	}
}

func TestLoadDotEnv_SkipsComments(t *testing.T) {
	env, err := LoadDotEnv("../../testdata/config", "valid-env-file.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for key := range env {
		if key == "" || key[0] == '#' {
			t.Errorf("comment or empty key appeared: %q", key)
		}
	}
}

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
