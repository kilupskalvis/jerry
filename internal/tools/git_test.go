package tools_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/motif/internal/tools"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "checkout", "-b", "main")

	if err := os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "initial commit")

	return dir
}

func TestGitLog_Success(t *testing.T) {
	repoRoot := setupGitRepo(t)
	tool := tools.NewGitLogTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "initial commit") {
		t.Errorf("expected log to contain 'initial commit', got: %s", result)
	}
}

func TestGitLog_WithPath(t *testing.T) {
	repoRoot := setupGitRepo(t)
	tool := tools.NewGitLogTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{"path": "hello.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "initial commit") {
		t.Errorf("expected log to contain commit, got: %s", result)
	}
}

func TestGitLog_CountClamping(t *testing.T) {
	repoRoot := setupGitRepo(t)
	tool := tools.NewGitLogTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{"count": float64(100)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGitLog_NotARepo(t *testing.T) {
	tool := tools.NewGitLogTool(t.TempDir())
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	lower := strings.ToLower(result)
	if !strings.Contains(lower, "error") && !strings.Contains(lower, "fatal") {
		t.Errorf("expected error message for non-repo, got: %s", result)
	}
}

func TestGitDiff_NoChanges(t *testing.T) {
	repoRoot := setupGitRepo(t)
	tool := tools.NewGitDiffTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No changes found") {
		t.Errorf("expected 'No changes found', got: %s", result)
	}
}

func TestGitDiff_WithChanges(t *testing.T) {
	repoRoot := setupGitRepo(t)
	_ = os.WriteFile(filepath.Join(repoRoot, "hello.go"),
		[]byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644)

	tool := tools.NewGitDiffTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello.go") {
		t.Errorf("expected diff to mention hello.go, got: %s", result)
	}
}

func TestGitDiff_WithRef(t *testing.T) {
	repoRoot := setupGitRepo(t)
	_ = os.WriteFile(filepath.Join(repoRoot, "hello.go"),
		[]byte("package main\n\nfunc main() { /* changed */ }\n"), 0o644)

	tool := tools.NewGitDiffTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{"ref": "HEAD"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "changed") {
		t.Errorf("expected diff against HEAD, got: %s", result)
	}
}

func TestGitBlame_Success(t *testing.T) {
	repoRoot := setupGitRepo(t)
	tool := tools.NewGitBlameTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{"path": "hello.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "package main") {
		t.Errorf("expected blame to contain file content, got: %s", result)
	}
}

func TestGitBlame_LineRange(t *testing.T) {
	repoRoot := setupGitRepo(t)
	tool := tools.NewGitBlameTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "hello.go", "start_line": float64(1), "end_line": float64(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "package main") {
		t.Errorf("expected blame line 1, got: %s", result)
	}
}

func TestGitBlame_PathTraversal(t *testing.T) {
	repoRoot := setupGitRepo(t)
	tool := tools.NewGitBlameTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{"path": "../../etc/passwd"})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !strings.Contains(strings.ToLower(result), "escapes") {
		t.Errorf("expected path escape error, got: %s", result)
	}
}

func TestGitBlame_MissingPath(t *testing.T) {
	repoRoot := setupGitRepo(t)
	tool := tools.NewGitBlameTool(repoRoot)
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !strings.Contains(strings.ToLower(result), "required") {
		t.Errorf("expected missing path error, got: %s", result)
	}
}

func TestGitTools_RegisteredInRegistry(t *testing.T) {
	reg := tools.NewRegistry(t.TempDir(), nil)
	names := reg.KnownToolNames()

	want := []string{"git_log", "git_diff", "git_blame"}
	for _, name := range want {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tool %q not found in registry", name)
		}
	}
}
