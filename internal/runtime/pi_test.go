package runtime

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/spec"
)

func writeFakePi(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pi")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPiInvokeSpawnsAndParses(t *testing.T) {
	line := `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"spawned ok"}],"usage":{"input":2,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":3,"cost":{"total":0.01}},"stopReason":"stop"}}`
	dir := writeFakePi(t, "cat <<'EOF'\n"+line+"\nEOF")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	pi := NewPi(PiOptions{})
	res, err := pi.Invoke(context.Background(), InvocationSpec{Prompt: "hi", Model: "claude-haiku-4-5"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.Text != "spawned ok" || res.Usage == nil || res.Usage.CostUSD != 0.01 {
		t.Errorf("result = %+v usage=%+v", res, res.Usage)
	}
}

func TestPiInvokeNonZeroExit(t *testing.T) {
	dir := writeFakePi(t, "echo 'boom' >&2; exit 1")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	pi := NewPi(PiOptions{})
	if _, err := pi.Invoke(context.Background(), InvocationSpec{Prompt: "x", Model: "m"}); err == nil {
		t.Fatal("want error when pi exits non-zero")
	}
}

func TestPiName(t *testing.T) {
	if NewPi(PiOptions{}).Name() != "pi" {
		t.Error("Name must be pi")
	}
}

func TestPiPreflightMatch(t *testing.T) {
	dir := writeFakePi(t, `[ "$1" = "--version" ] && { echo "0.73.1"; exit 0; }; cat <<'EOF'
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":2,"cost":{"total":0}},"stopReason":"stop"}}
EOF`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	pi := NewPi(PiOptions{PinnedVersion: "0.73.1"})
	if _, err := pi.Invoke(context.Background(), InvocationSpec{Prompt: "x", Model: "m"}); err != nil {
		t.Fatalf("matching version should pass: %v", err)
	}
}

func TestPiPreflightMismatch(t *testing.T) {
	dir := writeFakePi(t, `[ "$1" = "--version" ] && { echo "0.99.0"; exit 0; }; echo unused`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	pi := NewPi(PiOptions{PinnedVersion: "0.73.1"})
	_, err := pi.Invoke(context.Background(), InvocationSpec{Prompt: "x", Model: "m"})
	if err == nil {
		t.Fatal("version mismatch must error")
	}
	if !strings.Contains(err.Error(), "0.73.1") || !strings.Contains(err.Error(), "0.99.0") {
		t.Errorf("error should name both versions: %v", err)
	}
}

func TestPiPreflightMissingBinary(t *testing.T) {
	pi := NewPi(PiOptions{PinnedVersion: "0.73.1", Binary: "definitely-not-a-real-binary-xyz"})
	if _, err := pi.Invoke(context.Background(), InvocationSpec{Prompt: "x"}); err == nil {
		t.Fatal("missing pi binary must error")
	}
}

func TestBuildArgs(t *testing.T) {
	args := buildArgs(InvocationSpec{
		Prompt:      "do the thing",
		Model:       "claude-sonnet-4-6",
		Permissions: spec.PermissionSet{Allow: []string{"read", "bash(go test:*)"}},
	})
	want := []string{"--print", "--mode", "json", "--model", "claude-sonnet-4-6", "--tools", "read,bash", "do the thing"}
	if !slices.Equal(args, want) {
		t.Errorf("buildArgs =\n  %v\nwant\n  %v", args, want)
	}
}

func TestBuildArgsNoToolsWhenAllowEmpty(t *testing.T) {
	args := buildArgs(InvocationSpec{Prompt: "p", Model: "m"})
	if !slices.Contains(args, "--no-tools") {
		t.Errorf("empty allow must yield --no-tools, got %v", args)
	}
	if slices.Contains(args, "--tools") {
		t.Errorf("must not emit --tools with no allow: %v", args)
	}
}

func TestBuildArgsOmitsModelWhenEmpty(t *testing.T) {
	args := buildArgs(InvocationSpec{Prompt: "p"})
	if slices.Contains(args, "--model") {
		t.Errorf("must not emit --model when empty: %v", args)
	}
}

func TestParseSessionSuccess(t *testing.T) {
	data, err := os.ReadFile("testdata/pi-success.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseSession(data)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}
	if res.Text != "hello from jerry" {
		t.Errorf("Text = %q", res.Text)
	}
	if res.Usage == nil {
		t.Fatal("Usage nil")
	}
	if res.Usage.InputTokens != 150 || res.Usage.OutputTokens != 8 {
		t.Errorf("tokens = %d/%d, want 150/8", res.Usage.InputTokens, res.Usage.OutputTokens)
	}
	if res.Usage.CostUSD != 0.00163 {
		t.Errorf("cost = %v, want 0.00163", res.Usage.CostUSD)
	}
}

func TestParseSessionError(t *testing.T) {
	data, err := os.ReadFile("testdata/pi-error.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, err = parseSession(data)
	if err == nil {
		t.Fatal("want error for stopReason=error session")
	}
	if !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Errorf("error should surface pi errorMessage, got: %v", err)
	}
}

func TestParseSessionMultilineConcatsTextOnly(t *testing.T) {
	data, err := os.ReadFile("testdata/pi-multiline.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseSession(data)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}
	if res.Text != "line one\nline two" {
		t.Errorf("Text = %q, want two text blocks joined by newline", res.Text)
	}
}

func TestParseSessionNoAssistant(t *testing.T) {
	_, err := parseSession([]byte(`{"type":"session","version":1}` + "\n" + `{"type":"agent_end","messages":[]}` + "\n"))
	if err == nil {
		t.Fatal("want error when no assistant message present")
	}
}

func TestParseSessionSkipsBlankAndBadLines(t *testing.T) {
	good := `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":2,"cost":{"total":0}},"stopReason":"stop"}}`
	res, err := parseSession([]byte("\n" + good + "\n\n"))
	if err != nil {
		t.Fatalf("blank lines should be tolerated: %v", err)
	}
	if res.Text != "ok" {
		t.Errorf("Text = %q", res.Text)
	}
	if _, err := parseSession([]byte("not json\n")); err == nil {
		t.Fatal("want error on non-JSON line")
	}
}

func TestPermsToToolFlags(t *testing.T) {
	cases := []struct {
		name  string
		allow []string
		want  []string
	}{
		{"nouns deduped", []string{"read", "bash(go test:*)", "bash(go vet:*)", "edit"}, []string{"read", "bash", "edit"}},
		{"write selector", []string{"write(.jerry/x)"}, []string{"write"}},
		{"empty", nil, nil},
		{"unknown noun dropped", []string{"read", "telepathy"}, []string{"read"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := permsToToolNames(spec.PermissionSet{Allow: tc.allow})
			if !slices.Equal(got, tc.want) {
				t.Errorf("permsToToolNames(%v) = %v, want %v", tc.allow, got, tc.want)
			}
		})
	}
}
