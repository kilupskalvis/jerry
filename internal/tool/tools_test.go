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

func TestReadFile_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello\nworld"), 0o644)

	rf := tool.NewReadFileTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "test.txt"})
	result, err := rf.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "1: hello") {
		t.Errorf("got %q, want line-numbered output", result)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	t.Parallel()
	rf := tool.NewReadFileTool(t.TempDir())
	input, _ := json.Marshal(map[string]string{"path": "missing.txt"})
	result, _ := rf.Execute(context.Background(), input)
	if !strings.Contains(result, "not found") {
		t.Errorf("got %q, want not found error", result)
	}
}

func TestReadFile_PathEscape(t *testing.T) {
	t.Parallel()
	rf := tool.NewReadFileTool(t.TempDir())
	input, _ := json.Marshal(map[string]string{"path": "../../etc/passwd"})
	result, _ := rf.Execute(context.Background(), input)
	if !strings.Contains(result, "escapes") {
		t.Errorf("got %q, want path escape error", result)
	}
}

func TestWriteFile_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wf := tool.NewWriteFileTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "out.txt", "content": "hello"})
	result, err := wf.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("got %q, want success message", result)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "out.txt"))
	if string(data) != "hello" {
		t.Errorf("file content = %q, want 'hello'", data)
	}
}

func TestWriteFile_CreatesDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wf := tool.NewWriteFileTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "a/b/c.txt", "content": "deep"})
	result, _ := wf.Execute(context.Background(), input)
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("got %q, want success message", result)
	}
}

func TestWriteFile_PathEscape(t *testing.T) {
	t.Parallel()
	wf := tool.NewWriteFileTool(t.TempDir())
	input, _ := json.Marshal(map[string]string{"path": "../../evil.txt", "content": "hack"})
	result, _ := wf.Execute(context.Background(), input)
	if !strings.Contains(result, "escapes") {
		t.Errorf("got %q, want path escape error", result)
	}
}

func TestRegistry_BaseTools(t *testing.T) {
	t.Parallel()
	reg := tool.NewRegistry(t.TempDir(), nil)
	base := reg.BaseTools()
	names := make(map[string]bool)
	for _, bt := range base {
		names[bt.Name()] = true
	}
	for _, want := range []string{"bash", "read_file", "write_file"} {
		if !names[want] {
			t.Errorf("base tools missing %q", want)
		}
	}
}

func TestRegistry_ResolveCITool(t *testing.T) {
	t.Parallel()
	reg := tool.NewRegistry(t.TempDir(), nil)
	resolved, err := reg.Resolve([]tool.ToolAccess{{Name: "post_pr_comment"}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved) != 1 || resolved[0].Name() != "post_pr_comment" {
		t.Errorf("got %v, want [post_pr_comment]", resolved)
	}
}

func TestRegistry_ResolveUnknown(t *testing.T) {
	t.Parallel()
	reg := tool.NewRegistry(t.TempDir(), nil)
	_, err := reg.Resolve([]tool.ToolAccess{{Name: "nonexistent"}})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRegistry_ResolveSkipsBaseTools(t *testing.T) {
	t.Parallel()
	reg := tool.NewRegistry(t.TempDir(), nil)
	resolved, err := reg.Resolve([]tool.ToolAccess{{Name: "bash"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 0 {
		t.Errorf("got %d resolved, want 0 (bash is base tool)", len(resolved))
	}
}
