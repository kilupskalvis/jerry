package runtime

import (
	"os"
	"testing"

	"github.com/kilupskalvis/jerry/internal/spec"
)

func TestBuildClaudeCodeArgs(t *testing.T) {
	args := buildClaudeCodeArgs(InvocationSpec{
		Prompt: "review this",
		Model:  "claude-sonnet-4-6",
		Permissions: spec.PermissionSet{
			Allow: []string{"read", "bash(go test:*)"},
			Deny:  []string{"bash(rm:*)"},
		},
	})

	want := []string{"-p", "review this", "--output-format", "json",
		"--model", "claude-sonnet-4-6",
		"--allowedTools", "Read,Bash",
		"--disallowedTools", "Bash"}

	if len(args) != len(want) {
		t.Fatalf("args = %v\nwant %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestBuildClaudeCodeArgsNoPerms(t *testing.T) {
	args := buildClaudeCodeArgs(InvocationSpec{Prompt: "hi"})
	for _, a := range args {
		if a == "--allowedTools" || a == "--disallowedTools" {
			t.Errorf("should not have tool flags without permissions: %v", args)
			break
		}
	}
}

func TestParseClaudeCodeSuccess(t *testing.T) {
	data, err := os.ReadFile("testdata/claude-code-success.json")
	if err != nil {
		t.Fatal(err)
	}
	res, parseErr := parseClaudeCodeOutput(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if res.Text == "" {
		t.Error("empty text")
	}
	if res.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if res.Usage.CostUSD != 0.042 {
		t.Errorf("CostUSD = %v", res.Usage.CostUSD)
	}
	if res.Usage.InputTokens != 1500 {
		t.Errorf("InputTokens = %d", res.Usage.InputTokens)
	}
	if res.Usage.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d", res.Usage.OutputTokens)
	}
}

func TestParseClaudeCodeError(t *testing.T) {
	data, err := os.ReadFile("testdata/claude-code-error.json")
	if err != nil {
		t.Fatal(err)
	}
	_, parseErr := parseClaudeCodeOutput(data)
	if parseErr == nil {
		t.Fatal("want error for is_error=true")
	}
}

func TestClaudeCodeName(t *testing.T) {
	cc := NewClaudeCode(ClaudeCodeOptions{})
	if cc.Name() != "claude-code" {
		t.Errorf("Name = %q", cc.Name())
	}
	caps := cc.Capabilities()
	if !caps.CostReporting {
		t.Error("CostReporting should be true")
	}
	if !caps.Permissions {
		t.Error("Permissions should be true")
	}
}
