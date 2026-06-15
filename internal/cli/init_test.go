package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/cli"
	"github.com/kilupskalvis/jerry/internal/spec"
)

func initInto(t *testing.T, args ...string) string {
	t.Helper()
	tmpDir := t.TempDir()
	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs(append([]string{"init", "--path", tmpDir}, args...))
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	return tmpDir
}

func TestInitCmd_CreatesDirectory(t *testing.T) {
	tmpDir := initInto(t)
	info, statErr := os.Stat(filepath.Join(tmpDir, ".jerry"))
	if statErr != nil || !info.IsDir() {
		t.Fatalf(".jerry/ not created as a directory: %v", statErr)
	}
}

func TestInitCmd_CreatesReviewWorkflow(t *testing.T) {
	tmpDir := initInto(t)
	content, readErr := os.ReadFile(filepath.Join(tmpDir, ".jerry", "review", "workflow.yaml"))
	if readErr != nil {
		t.Fatalf("review/workflow.yaml not created: %v", readErr)
	}
	if !strings.Contains(string(content), "version: 1") {
		t.Error("workflow.yaml should be a v3 spec (version: 1)")
	}
	if !strings.Contains(string(content), "prompt: reviewer.md") {
		t.Error("workflow.yaml should reference reviewer.md")
	}
}

func TestInitCmd_CreatesReviewPrompt(t *testing.T) {
	tmpDir := initInto(t)
	content, readErr := os.ReadFile(filepath.Join(tmpDir, ".jerry", "review", "reviewer.md"))
	if readErr != nil {
		t.Fatalf("review/reviewer.md not created: %v", readErr)
	}
	if strings.HasPrefix(strings.TrimSpace(string(content)), "---") {
		t.Error("v3 prompt files must not have YAML frontmatter")
	}
	if !strings.Contains(string(content), "Output Contract") {
		t.Error("reviewer.md should declare its output contract")
	}
}

func TestInitCmd_ScaffoldValidates(t *testing.T) {
	tmpDir := initInto(t)
	project, err := spec.LoadProject(filepath.Join(tmpDir, ".jerry"))
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if issues := spec.ValidateProject(project); spec.HasErrors(issues) {
		t.Errorf("scaffold must validate clean, got: %v", issues)
	}
}

func TestInitCmd_FeatureTemplateValidates(t *testing.T) {
	tmpDir := initInto(t)
	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir, "--template", "feature"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init --template feature failed: %v", err)
	}
	project, err := spec.LoadProject(filepath.Join(tmpDir, ".jerry"))
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(project.Workflows) != 2 {
		t.Fatalf("want review + feature workflows, got %d", len(project.Workflows))
	}
	if issues := spec.ValidateProject(project); spec.HasErrors(issues) {
		t.Errorf("review + feature scaffold must validate clean: %v", issues)
	}
}

func TestInitCmd_JerryGitignore(t *testing.T) {
	tmpDir := initInto(t)
	content, readErr := os.ReadFile(filepath.Join(tmpDir, ".jerry", ".gitignore"))
	if readErr != nil {
		t.Fatalf(".jerry/.gitignore not created: %v", readErr)
	}
	if !strings.Contains(string(content), "settings.local.yaml") {
		t.Error(".jerry/.gitignore should include settings.local.yaml")
	}
}

func TestInitCmd_RootGitignoreIgnoresCtxDir(t *testing.T) {
	tmpDir := initInto(t)
	content, readErr := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	if readErr != nil {
		t.Fatalf("root .gitignore not created: %v", readErr)
	}
	if !strings.Contains(string(content), ".jerry-run/") {
		t.Error("root .gitignore should ignore .jerry-run/")
	}
}

func TestInitCmd_RootGitignoreAppendsWithoutClobber(t *testing.T) {
	tmpDir := t.TempDir()
	gitignore := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	content, _ := os.ReadFile(gitignore)
	if !strings.Contains(string(content), "node_modules/") {
		t.Error("existing .gitignore content was clobbered")
	}
	if !strings.Contains(string(content), ".jerry-run/") {
		t.Error(".jerry-run/ not appended")
	}
}

func TestInitCmd_CreatesSettingsYAML(t *testing.T) {
	tmpDir := initInto(t)
	data, err := os.ReadFile(filepath.Join(tmpDir, ".jerry", "settings.yaml"))
	if err != nil {
		t.Fatalf("settings.yaml not created: %v", err)
	}
	if !strings.Contains(string(data), "policy:") {
		t.Error("settings.yaml should contain a policy block")
	}
	if !strings.Contains(string(data), "rm -rf") {
		t.Error("settings.yaml should contain default deny rules")
	}
}

func TestInitCmd_AlreadyExists(t *testing.T) {
	tmpDir := initInto(t)
	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error when .jerry/ already exists")
	}
}
