package github

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kilupskalvis/jerry/internal/compile"
	"github.com/kilupskalvis/jerry/internal/spec"
)

var update = flag.Bool("update", false, "update golden files")

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	golden := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", golden)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to generate): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n--- got ---\n%s\n--- want ---\n%s",
			golden, got, want)
	}
}

func minimalPlan() *compile.Plan {
	return &compile.Plan{
		JerryVersion: "0.1.0",
		Files: []compile.PlannedFile{
			{
				Path: ".github/workflows/jerry-minimal.yml",
				Jobs: []compile.PlannedJob{
					{
						Name:     "minimal",
						Triggers: spec.Triggers{Push: &spec.PushTrigger{Branches: []string{"main"}}},
						Env:      []string{"ANTHROPIC_API_KEY"},
						Steps: []compile.PlannedStep{
							{Label: "Checkout", Command: "actions/checkout@v4", IsPreamble: true},
							{Label: "Install Jerry", Command: "curl -sSL https://jerry.dev/install.sh | sh -s -- --version v0.1.0", IsPreamble: true},
							{Label: "Drift check", Command: "jerry generate --check", IsPreamble: true},
							{Label: "greet", Command: "jerry exec minimal/greet", EnvRefs: []compile.EnvRef{
								{Name: "ANTHROPIC_API_KEY", SecretRef: "${{ secrets.ANTHROPIC_API_KEY }}"},
							}},
						},
					},
				},
			},
		},
	}
}

func TestEmitMinimal(t *testing.T) {
	files, err := Emit(minimalPlan())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	assertGolden(t, "minimal.yml", files[0].Content)
}

func TestEmitDeterministic(t *testing.T) {
	plan := minimalPlan()
	f1, _ := Emit(plan)
	f2, _ := Emit(plan)
	if string(f1[0].Content) != string(f2[0].Content) {
		t.Error("Emit is not deterministic")
	}
}
