package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAdapters(t *testing.T) {
	dir := t.TempDir()
	adapterDir := filepath.Join(dir, "adapters")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `
name: goose
command: goose run --quiet --output-format json
prompt: stdin
parse:
  text: "result.text"
  cost: "usage.cost"
capabilities:
  structured_output: false
  cost_reporting: true
  permissions: false
`
	if err := os.WriteFile(filepath.Join(adapterDir, "goose.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	adapters, err := LoadAdapters(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) != 1 {
		t.Fatalf("len = %d", len(adapters))
	}
	a := adapters[0]
	if a.Name != "goose" {
		t.Errorf("Name = %q", a.Name)
	}
	if a.Command != "goose run --quiet --output-format json" {
		t.Errorf("Command = %q", a.Command)
	}
	if a.Prompt != "stdin" {
		t.Errorf("Prompt = %q", a.Prompt)
	}
	if a.Parse.Text != "result.text" {
		t.Errorf("Parse.Text = %q", a.Parse.Text)
	}
	if a.Parse.Cost != "usage.cost" {
		t.Errorf("Parse.Cost = %q", a.Parse.Cost)
	}
	if !a.Capabilities.CostReporting {
		t.Error("CostReporting should be true")
	}
}

func TestLoadAdaptersEmpty(t *testing.T) {
	dir := t.TempDir()
	adapters, err := LoadAdapters(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) != 0 {
		t.Errorf("len = %d, want 0", len(adapters))
	}
}

func TestLoadAdaptersMissingName(t *testing.T) {
	dir := t.TempDir()
	adapterDir := filepath.Join(dir, "adapters")
	os.MkdirAll(adapterDir, 0o755)
	os.WriteFile(filepath.Join(adapterDir, "bad.yaml"), []byte("command: foo\nprompt: arg\nparse:\n  text: x\n"), 0o644)

	_, err := LoadAdapters(dir)
	if err == nil {
		t.Fatal("want error for adapter without name")
	}
}

func TestLoadAdaptersSorted(t *testing.T) {
	dir := t.TempDir()
	adapterDir := filepath.Join(dir, "adapters")
	os.MkdirAll(adapterDir, 0o755)
	for _, name := range []string{"zeta", "alpha"} {
		y := "name: " + name + "\ncommand: " + name + "\nprompt: arg\nparse:\n  text: out\n"
		os.WriteFile(filepath.Join(adapterDir, name+".yaml"), []byte(y), 0o644)
	}

	adapters, err := LoadAdapters(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) != 2 || adapters[0].Name != "alpha" {
		t.Errorf("not sorted: %v", adapters)
	}
}
