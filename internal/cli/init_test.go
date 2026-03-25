package cli_test

import (
	"os"
	"path/filepath"
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

func TestInitCmd_CreatesExamplePipeline(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	pipelinePath := filepath.Join(tmpDir, ".jerry", "pipelines", "example.yaml")
	if _, statErr := os.Stat(pipelinePath); statErr != nil {
		t.Fatalf("example.yaml not created: %v", statErr)
	}

	content, _ := os.ReadFile(pipelinePath)
	if len(content) == 0 {
		t.Error("example.yaml should not be empty")
	}
}

func TestInitCmd_CreatesScripts(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	scriptPath := filepath.Join(tmpDir, ".jerry", "scripts", "echo-context.sh")
	info, statErr := os.Stat(scriptPath)
	if statErr != nil {
		t.Fatalf("echo-context.sh not created: %v", statErr)
	}

	// Should be executable
	if info.Mode()&0o111 == 0 {
		t.Error("echo-context.sh should be executable")
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

	contentStr := string(content)
	if !contains(contentStr, "runs/") {
		t.Error(".gitignore should contain 'runs/'")
	}
	if !contains(contentStr, "cache/") {
		t.Error(".gitignore should contain 'cache/'")
	}
}

func TestInitCmd_CreatesAgents(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	agentsDir := filepath.Join(tmpDir, ".jerry", "agents")
	for _, name := range []string{"plan.md", "generate.md"} {
		path := filepath.Join(agentsDir, name)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("agents/%s not created: %v", name, statErr)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("agents/%s should not be empty", name)
		}
	}
}

func TestInitCmd_CreatesFeaturePipeline(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	featurePath := filepath.Join(tmpDir, ".jerry", "pipelines", "feature.yaml")
	content, readErr := os.ReadFile(featurePath)
	if readErr != nil {
		t.Fatalf("feature.yaml not created: %v", readErr)
	}
	if !contains(string(content), "generate") {
		t.Error("feature.yaml should reference generate agent")
	}
}

func TestInitCmd_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	// First init
	rootCmd := cli.NewRootCmd(&cli.App{})
	rootCmd.SetArgs([]string{"init", "--path", tmpDir})
	_ = rootCmd.Execute()

	// Second init should fail
	rootCmd2 := cli.NewRootCmd(&cli.App{})
	rootCmd2.SetArgs([]string{"init", "--path", tmpDir})
	err := rootCmd2.Execute()
	if err == nil {
		t.Fatal("expected error when .jerry/ already exists")
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
