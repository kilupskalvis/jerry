package tool_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/tool"
)

func setupRepoRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mustJSON marshals a map to json.RawMessage for tool Execute calls.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return data
}

// --- read_file tests ---

func TestReadFile_Success(t *testing.T) {
	root := setupRepoRoot(t)
	writeFile(t, root, "hello.txt", "line one\nline two\nline three")

	tt := tool.NewReadFileTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "hello.txt"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "1: line one") {
		t.Errorf("expected line numbers, got %q", result)
	}
	if !strings.Contains(result, "2: line two") {
		t.Errorf("expected line 2, got %q", result)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewReadFileTool(root)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "nonexistent.go"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Error: file not found") {
		t.Errorf("expected 'file not found' error, got %q", result)
	}
}

func TestReadFile_Directory(t *testing.T) {
	root := setupRepoRoot(t)
	os.MkdirAll(filepath.Join(root, "subdir"), 0o755)

	tt := tool.NewReadFileTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "subdir"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "is a directory") {
		t.Errorf("expected directory error, got %q", result)
	}
}

func TestReadFile_TooLarge(t *testing.T) {
	root := setupRepoRoot(t)
	// Create a file larger than 1MB.
	large := strings.Repeat("x", tool.MaxFileReadSize+100)
	writeFile(t, root, "large.txt", large)

	tt := tool.NewReadFileTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "large.txt"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "[truncated") {
		t.Errorf("expected truncation note, got last 100 chars: %q", result[len(result)-100:])
	}
}

func TestReadFile_PathEscape(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewReadFileTool(root)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "../../etc/passwd"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "escapes repository root") {
		t.Errorf("expected path escape error, got %q", result)
	}
}

// --- list_directory tests ---

func TestListDir_Success(t *testing.T) {
	root := setupRepoRoot(t)
	os.MkdirAll(filepath.Join(root, "subdir"), 0o755)
	writeFile(t, root, "file.go", "package main")
	writeFile(t, root, "readme.md", "# readme")

	tt := tool.NewListDirectoryTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "."}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "[dir]  subdir") {
		t.Errorf("expected [dir] subdir, got %q", result)
	}
	if !strings.Contains(result, "[file] file.go") {
		t.Errorf("expected [file] file.go, got %q", result)
	}
}

func TestListDir_NotDirectory(t *testing.T) {
	root := setupRepoRoot(t)
	writeFile(t, root, "file.txt", "content")

	tt := tool.NewListDirectoryTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "file.txt"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "is not a directory") {
		t.Errorf("expected 'not a directory' error, got %q", result)
	}
}

func TestListDir_PathEscape(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewListDirectoryTool(root)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "../../"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "escapes repository root") {
		t.Errorf("expected path escape error, got %q", result)
	}
}

func TestListDir_SortOrder(t *testing.T) {
	root := setupRepoRoot(t)
	os.MkdirAll(filepath.Join(root, "zdir"), 0o755)
	os.MkdirAll(filepath.Join(root, "adir"), 0o755)
	writeFile(t, root, "zfile.go", "")
	writeFile(t, root, "afile.go", "")

	tt := tool.NewListDirectoryTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"path": "."}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(result), "\n")
	// Dirs should come first: adir, zdir, then files: afile.go, zfile.go.
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "adir") {
		t.Errorf("first line should be adir, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "zdir") {
		t.Errorf("second line should be zdir, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "afile.go") {
		t.Errorf("third line should be afile.go, got %q", lines[2])
	}
}

// --- write_file tests ---

func TestWriteFile_Success(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewWriteFileTool(root)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{
		"path":    "output.txt",
		"content": "hello world",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("expected success message, got %q", result)
	}

	data, readErr := os.ReadFile(filepath.Join(root, "output.txt"))
	if readErr != nil {
		t.Fatalf("file not written: %v", readErr)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestWriteFile_CreatesDirectories(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewWriteFileTool(root)

	_, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{
		"path":    "deep/nested/dir/file.go",
		"content": "package deep",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, readErr := os.ReadFile(filepath.Join(root, "deep", "nested", "dir", "file.go"))
	if readErr != nil {
		t.Fatalf("file not written: %v", readErr)
	}
	if string(data) != "package deep" {
		t.Errorf("expected 'package deep', got %q", string(data))
	}
}

func TestWriteFile_PathEscape(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewWriteFileTool(root)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{
		"path":    "../../evil.txt",
		"content": "bad",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "escapes repository root") {
		t.Errorf("expected path escape error, got %q", result)
	}
}

// --- glob tests ---

func TestGlob_Matches(t *testing.T) {
	root := setupRepoRoot(t)
	writeFile(t, root, "main.go", "package main")
	writeFile(t, root, "util.go", "package util")
	writeFile(t, root, "readme.md", "# readme")

	tt := tool.NewGlobTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"pattern": "*.go"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("expected main.go in results, got %q", result)
	}
	if !strings.Contains(result, "util.go") {
		t.Errorf("expected util.go in results, got %q", result)
	}
	if strings.Contains(result, "readme.md") {
		t.Errorf("readme.md should not match *.go pattern")
	}
}

func TestGlob_NoMatches(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewGlobTool(root)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"pattern": "*.xyz"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No files matched") {
		t.Errorf("expected 'No files matched', got %q", result)
	}
}

func TestGlob_DoublestarPattern(t *testing.T) {
	root := setupRepoRoot(t)
	writeFile(t, root, "src/a.go", "package a")
	writeFile(t, root, "src/sub/b.go", "package b")
	writeFile(t, root, "test.txt", "test")

	tt := tool.NewGlobTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"pattern": "**/*.go"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "src/a.go") {
		t.Errorf("expected src/a.go, got %q", result)
	}
	if !strings.Contains(result, "src/sub/b.go") {
		t.Errorf("expected src/sub/b.go, got %q", result)
	}
	if strings.Contains(result, "test.txt") {
		t.Errorf("test.txt should not match **/*.go")
	}
}

// --- search_codebase tests ---

func TestSearch_Matches(t *testing.T) {
	root := setupRepoRoot(t)
	writeFile(t, root, "main.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}")

	tt := tool.NewSearchTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"query": "func main"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "main.go:3:") {
		t.Errorf("expected match at main.go:3, got %q", result)
	}
}

func TestSearch_NoMatches(t *testing.T) {
	root := setupRepoRoot(t)
	writeFile(t, root, "main.go", "package main")

	tt := tool.NewSearchTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"query": "nonexistent_function"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No matches found") {
		t.Errorf("expected 'No matches found', got %q", result)
	}
}

func TestSearch_InvalidRegex(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewSearchTool(root)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"query": "[invalid"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Error: invalid regex") {
		t.Errorf("expected regex error, got %q", result)
	}
}

func TestSearch_SkipsBinaryFiles(t *testing.T) {
	root := setupRepoRoot(t)
	// Write a binary file (contains null bytes).
	writeFile(t, root, "binary.dat", "hello\x00world")
	writeFile(t, root, "text.txt", "hello world")

	tt := tool.NewSearchTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"query": "hello"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "binary.dat") {
		t.Error("binary files should be skipped")
	}
	if !strings.Contains(result, "text.txt") {
		t.Errorf("text.txt should be in results, got %q", result)
	}
}

func TestSearch_WithGlobFilter(t *testing.T) {
	root := setupRepoRoot(t)
	writeFile(t, root, "main.go", "func hello() {}")
	writeFile(t, root, "main.py", "def hello():")

	tt := tool.NewSearchTool(root)
	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{
		"query": "hello",
		"glob":  "*.go",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("expected main.go in results, got %q", result)
	}
	if strings.Contains(result, "main.py") {
		t.Error("main.py should be filtered out by *.go glob")
	}
}

// --- run_command tests ---

func TestRunCommand_Success(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewRunCommandTool(root, nil)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"command": "echo hello"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result) != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestRunCommand_Failure(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewRunCommandTool(root, nil)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"command": "exit 1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Command failed") {
		t.Errorf("expected 'Command failed', got %q", result)
	}
	if !strings.Contains(result, "exit code 1") {
		t.Errorf("expected exit code 1, got %q", result)
	}
}

func TestRunCommand_WorkingDirectory(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewRunCommandTool(root, nil)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"command": "pwd"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve symlinks for comparison (macOS /tmp is a symlink to /private/tmp).
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	resolvedResult, _ := filepath.EvalSymlinks(strings.TrimSpace(result))

	if resolvedResult != resolvedRoot {
		t.Errorf("expected working directory %q, got %q", resolvedRoot, resolvedResult)
	}
}

func TestRunCommand_NoOutput(t *testing.T) {
	root := setupRepoRoot(t)
	tt := tool.NewRunCommandTool(root, nil)

	result, err := tt.Execute(context.Background(), mustJSON(t, map[string]any{"command": "true"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "(no output)" {
		t.Errorf("expected '(no output)', got %q", result)
	}
}

// --- constraint tests ---

func TestConstraint_RestrictTo_Allowed(t *testing.T) {
	root := setupRepoRoot(t)
	result := tool.ValidateConstraints("write_file",
		map[string]any{"path": "src/main.go"},
		map[string]any{"restrict_to": []any{"src/"}},
		root,
	)
	if result != "" {
		t.Errorf("expected no violation, got %q", result)
	}
}

func TestConstraint_RestrictTo_Blocked(t *testing.T) {
	root := setupRepoRoot(t)
	result := tool.ValidateConstraints("write_file",
		map[string]any{"path": "config/secret.yaml"},
		map[string]any{"restrict_to": []any{"src/", "tests/"}},
		root,
	)
	if !strings.Contains(result, "write_file blocked") {
		t.Errorf("expected violation, got %q", result)
	}
}

func TestConstraint_RestrictTo_PathTraversal(t *testing.T) {
	root := setupRepoRoot(t)
	result := tool.ValidateConstraints("write_file",
		map[string]any{"path": "src/../../etc/passwd"},
		map[string]any{"restrict_to": []any{"src/"}},
		root,
	)
	if result == "" {
		t.Error("expected violation for path traversal")
	}
}

func TestConstraint_RunCommand_Allowed(t *testing.T) {
	result := tool.ValidateConstraints("run_command",
		map[string]any{"command": "go test ./..."},
		map[string]any{"allow": []any{"go test"}},
		"",
	)
	if result != "" {
		t.Errorf("expected no violation, got %q", result)
	}
}

func TestConstraint_RunCommand_Denied(t *testing.T) {
	result := tool.ValidateConstraints("run_command",
		map[string]any{"command": "rm -rf /"},
		map[string]any{"deny": []any{"rm"}},
		"",
	)
	if !strings.Contains(result, "run_command blocked") {
		t.Errorf("expected violation, got %q", result)
	}
}

func TestConstraint_RunCommand_DenyBeforeAllow(t *testing.T) {
	result := tool.ValidateConstraints("run_command",
		map[string]any{"command": "rm -rf /"},
		map[string]any{
			"allow": []any{"rm"},
			"deny":  []any{"rm"},
		},
		"",
	)
	if !strings.Contains(result, "deny rule") {
		t.Errorf("deny should take precedence over allow, got %q", result)
	}
}

func TestConstraint_RunCommand_ShellOperator(t *testing.T) {
	result := tool.ValidateConstraints("run_command",
		map[string]any{"command": "go test && rm -rf /"},
		map[string]any{
			"allow": []any{"go test"},
			"deny":  []any{"rm"},
		},
		"",
	)
	if !strings.Contains(result, "rm -rf /") {
		t.Errorf("expected 'rm -rf /' blocked, got %q", result)
	}
}

func TestConstraint_RunCommand_QuotedOperator(t *testing.T) {
	result := tool.ValidateConstraints("run_command",
		map[string]any{"command": `echo "a && b"`},
		map[string]any{"allow": []any{"echo"}},
		"",
	)
	if result != "" {
		t.Errorf("quoted && should not be treated as operator, got %q", result)
	}
}

func TestConstraint_RunCommand_NoConstraints(t *testing.T) {
	result := tool.ValidateConstraints("run_command",
		map[string]any{"command": "anything goes"},
		map[string]any{},
		"",
	)
	if result != "" {
		t.Errorf("expected no violation with no constraints, got %q", result)
	}
}

// --- registry tests ---

func TestResolve_AllTools(t *testing.T) {
	root := setupRepoRoot(t)
	reg := tool.NewRegistry(root, nil)

	allTools := []tool.ToolAccess{
		{Name: "read_file"},
		{Name: "write_file"},
		{Name: "glob"},
		{Name: "search_codebase"},
		{Name: "run_command"},
		{Name: "list_directory"},
	}

	resolved, err := reg.Resolve(allTools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 6 {
		t.Errorf("expected 6 tools, got %d", len(resolved))
	}
}

func TestResolve_SubsetTools(t *testing.T) {
	root := setupRepoRoot(t)
	reg := tool.NewRegistry(root, nil)

	resolved, err := reg.Resolve([]tool.ToolAccess{
		{Name: "read_file"},
		{Name: "glob"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 2 {
		t.Errorf("expected 2 tools, got %d", len(resolved))
	}
}

func TestResolve_UnknownTool(t *testing.T) {
	root := setupRepoRoot(t)
	reg := tool.NewRegistry(root, nil)

	_, err := reg.Resolve([]tool.ToolAccess{{Name: "nonexistent"}})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected 'unknown tool' error, got %q", err.Error())
	}
}

func TestKnownToolNames(t *testing.T) {
	root := setupRepoRoot(t)
	reg := tool.NewRegistry(root, nil)

	names := reg.KnownToolNames()
	if len(names) != 9 {
		t.Errorf("expected 9 known tools, got %d: %v", len(names), names)
	}
}
