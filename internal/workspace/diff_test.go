package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInRepo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInRepo(t, dir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-m", "init")
	return dir
}

func TestDiffTrackedModification(t *testing.T) {
	dir := gitRepo(t)
	pre, err := RecordState(dir)
	if err != nil {
		t.Fatalf("RecordState: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := Capture(dir, pre)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !strings.Contains(snap.Patch, "-one") || !strings.Contains(snap.Patch, "+two") {
		t.Errorf("patch missing change:\n%s", snap.Patch)
	}
	if snap.Stat == "" {
		t.Error("stat empty")
	}
}

func TestDiffUntrackedFile(t *testing.T) {
	dir := gitRepo(t)
	pre, _ := RecordState(dir)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := Capture(dir, pre)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !strings.Contains(snap.Patch, "new.txt") || !strings.Contains(snap.Patch, "+fresh") {
		t.Errorf("untracked file missing from patch:\n%s", snap.Patch)
	}
}

func TestDiffAgentCommitted(t *testing.T) {
	dir := gitRepo(t)
	pre, _ := RecordState(dir)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("committed change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-m", "agent commit")

	snap, err := Capture(dir, pre)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !strings.Contains(snap.Patch, "+committed change") {
		t.Errorf("committed change missing from patch:\n%s", snap.Patch)
	}
}

func TestDiffClean(t *testing.T) {
	dir := gitRepo(t)
	pre, _ := RecordState(dir)
	snap, err := Capture(dir, pre)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if snap.Patch != "" {
		t.Errorf("clean tree must produce empty patch, got:\n%s", snap.Patch)
	}
}
