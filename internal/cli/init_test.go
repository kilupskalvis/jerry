package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/cli"
)

func TestInitCmd_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	jerryDir := filepath.Join(tmpDir, ".jerry")
	info, statErr := os.Stat(jerryDir)
	if statErr != nil {
		t.Fatalf(".jerry/ not created: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatal(".jerry should be a directory")
	}
}

func TestInitCmd_CreatesReviewWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	workflowPath := filepath.Join(tmpDir, ".jerry", "review", "workflow.yaml")
	content, readErr := os.ReadFile(workflowPath)
	if readErr != nil {
		t.Fatalf("review/workflow.yaml not created: %v", readErr)
	}
	if !strings.Contains(string(content), "reviewer") {
		t.Error("workflow.yaml should reference reviewer agent")
	}
}

func TestInitCmd_CreatesReviewAgent(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	agentPath := filepath.Join(tmpDir, ".jerry", "review", "reviewer.md")
	content, readErr := os.ReadFile(agentPath)
	if readErr != nil {
		t.Fatalf("review/reviewer.md not created: %v", readErr)
	}
	if !strings.Contains(string(content), "name: reviewer") {
		t.Error("reviewer.md should have name: reviewer in frontmatter")
	}
}

func TestInitCmd_CreatesGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	gitignorePath := filepath.Join(tmpDir, ".jerry", ".gitignore")
	content, readErr := os.ReadFile(gitignorePath)
	if readErr != nil {
		t.Fatalf(".gitignore not created: %v", readErr)
	}

	if !strings.Contains(string(content), "runs/") {
		t.Error(".gitignore should contain 'runs/'")
	}
}

func TestInitCmd_CreatesRunsDir(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	runsDir := filepath.Join(tmpDir, ".jerry", "runs")
	info, statErr := os.Stat(runsDir)
	if statErr != nil {
		t.Fatalf("runs/ not created: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatal("runs/ should be a directory")
	}
}

func TestInitCmd_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	rootCmd2 := cli.NewRootCmd(&cli.App{})
	rootCmd2.SetArgs([]string{"init", "--path", tmpDir})
	err := rootCmd2.Execute()
	if err == nil {
		t.Fatal("expected error when .jerry/ already exists")
	}
}

func TestInitCmd_CreatesSettingsYAML(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	settingsPath := filepath.Join(tmpDir, ".jerry", "settings.yaml")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.yaml not created: %v", err)
	}
	if !strings.Contains(string(data), "permissions") {
		t.Error("settings.yaml should contain permissions block")
	}
	if !strings.Contains(string(data), "rm -rf") {
		t.Error("settings.yaml should contain default deny rules")
	}
}

func TestInitCmd_GitignoreIncludesSettingsLocal(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	gitignorePath := filepath.Join(tmpDir, ".jerry", ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf(".gitignore not created: %v", err)
	}
	if !strings.Contains(string(data), "settings.local.yaml") {
		t.Error(".gitignore should include settings.local.yaml")
	}
}

func TestInitCmd_WithPathFlag(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "subdir")
	if mkErr := os.MkdirAll(targetDir, 0o755); mkErr != nil {
		t.Fatalf("failed to create target dir: %v", mkErr)
	}

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", targetDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init with --path failed: %v", err)
	}

	jerryDir := filepath.Join(targetDir, ".jerry")
	if _, statErr := os.Stat(jerryDir); statErr != nil {
		t.Fatalf(".jerry/ not created in target dir: %v", statErr)
	}
}
