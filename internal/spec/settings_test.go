package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadSettings(t *testing.T) {
	dir := writeSettings(t, `
policy:
  deny: ["read(.env)", "bash(rm -rf:*)"]
  budget:
    max_cost_per_run: 10.00
  runtimes:
    allowed: [pi, claude-code]
`)
	s, err := LoadSettings(dir)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if len(s.Policy.Deny) != 2 || s.Policy.Budget.MaxCostPerRun != 10.0 ||
		len(s.Policy.Runtimes.Allowed) != 2 {
		t.Errorf("parsed settings wrong: %+v", s)
	}
}

func TestLoadSettingsAbsent(t *testing.T) {
	s, err := LoadSettings(t.TempDir())
	if err != nil {
		t.Fatalf("absent settings should not error: %v", err)
	}
	if s != nil {
		t.Errorf("want nil settings when file absent, got %+v", s)
	}
}

func TestLoadSettingsUnknownField(t *testing.T) {
	dir := writeSettings(t, "policy:\n  dny: []\n")
	if _, err := LoadSettings(dir); err == nil {
		t.Fatal("want error for unknown field")
	}
}
