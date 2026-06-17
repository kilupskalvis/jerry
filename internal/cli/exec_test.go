package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/output"
	"github.com/kilupskalvis/jerry/internal/runtime"
)

func fakePiOnPath(t *testing.T, sessionJSON string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do [ \"$a\" = \"--version\" ] && { echo 0.73.1; exit 0; }; done\ncat <<'JSON'\n" + sessionJSON + "\nJSON\n"
	if err := os.WriteFile(filepath.Join(dir, "pi"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestExecRunsSingleStep(t *testing.T) {
	repo, jerryDir := v3Project(t)
	fakePiOnPath(t, `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"planned"}],"usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":2,"cost":{"total":0}},"stopReason":"stop"}}`)

	app := &App{
		JerryDir: jerryDir, RepoRoot: repo,
		Registry: runtime.NewRegistry(runtime.NewPi(runtime.PiOptions{})),
		Printer:  output.NewPrinter(os.Stderr, os.Stderr),
	}
	if err := runExec(app, "demo/plan", execTrigger{intent: "ship it"}, false); err != nil {
		t.Fatalf("runExec: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(repo, ".jerry-run", "steps", "plan", "output.txt"))
	if readErr != nil || !strings.Contains(string(data), "planned") {
		t.Errorf("step output = %q err=%v", data, readErr)
	}
}

func TestExecBadStepRef(t *testing.T) {
	repo, jerryDir := v3Project(t)
	app := &App{JerryDir: jerryDir, RepoRoot: repo,
		Registry: runtime.NewRegistry(runtime.NewPi(runtime.PiOptions{})),
		Printer:  output.NewPrinter(os.Stderr, os.Stderr)}
	if err := runExec(app, "demo", execTrigger{}, false); err == nil {
		t.Fatal("want error for ref without a slash")
	}
}

func TestExecPropagatesStepExitCode(t *testing.T) {
	repo, jerryDir := v3Project(t)
	app := &App{JerryDir: jerryDir, RepoRoot: repo,
		Registry: runtime.NewRegistry(runtime.NewPi(runtime.PiOptions{})),
		Printer:  output.NewPrinter(os.Stderr, os.Stderr)}

	// A nonexistent workflow forces ExitConfig (2).
	err := runExec(app, "nope/plan", execTrigger{}, false)
	var ec interface{ ExitCode() int }
	if err == nil || !errors.As(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("want ExitCode 2, got err=%v", err)
	}
}

func TestExecSequentialHandoff(t *testing.T) {
	repo, jerryDir := v3Project(t)
	fakePiOnPath(t, `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"the plan"}],"usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":2,"cost":{"total":0}},"stopReason":"stop"}}`)

	app := &App{JerryDir: jerryDir, RepoRoot: repo,
		Registry: runtime.NewRegistry(runtime.NewPi(runtime.PiOptions{})),
		Printer:  output.NewPrinter(os.Stderr, os.Stderr)}

	if err := runExec(app, "demo/plan", execTrigger{intent: "ship it"}, false); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if err := runExec(app, "demo/echo", execTrigger{}, false); err != nil {
		t.Fatalf("step 2: %v", err)
	}

	out, _ := os.ReadFile(filepath.Join(repo, ".jerry-run", "steps", "echo", "output.txt"))
	if !strings.Contains(string(out), "got ship it") {
		t.Errorf("echo step did not see the trigger intent recorded by step 1: %q", out)
	}
}
