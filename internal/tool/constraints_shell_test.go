package tool

import (
	"testing"
)

func TestSplitShell_SingleCommand(t *testing.T) {
	result := splitShellCommands("go test ./...")
	assertSplit(t, result, []string{"go test ./..."})
}

func TestSplitShell_AndOperator(t *testing.T) {
	result := splitShellCommands("go test && go vet")
	assertSplit(t, result, []string{"go test", "go vet"})
}

func TestSplitShell_OrOperator(t *testing.T) {
	result := splitShellCommands("cmd1 || cmd2")
	assertSplit(t, result, []string{"cmd1", "cmd2"})
}

func TestSplitShell_Semicolon(t *testing.T) {
	result := splitShellCommands("cmd1; cmd2")
	assertSplit(t, result, []string{"cmd1", "cmd2"})
}

func TestSplitShell_Pipe(t *testing.T) {
	result := splitShellCommands("echo hi | grep hi")
	assertSplit(t, result, []string{"echo hi", "grep hi"})
}

func TestSplitShell_SingleQuoted(t *testing.T) {
	result := splitShellCommands("echo 'a && b'")
	assertSplit(t, result, []string{"echo 'a && b'"})
}

func TestSplitShell_DoubleQuoted(t *testing.T) {
	result := splitShellCommands(`echo "a && b"`)
	assertSplit(t, result, []string{`echo "a && b"`})
}

func TestSplitShell_MixedQuotesAndOps(t *testing.T) {
	result := splitShellCommands(`echo "hello" && rm -rf /`)
	assertSplit(t, result, []string{`echo "hello"`, "rm -rf /"})
}

func TestSplitShell_Subshell(t *testing.T) {
	result := splitShellCommands("echo $(whoami)")
	assertSplit(t, result, []string{"echo $(whoami)"})
}

func TestSplitShell_NestedSubshell(t *testing.T) {
	result := splitShellCommands("echo $(echo $(whoami))")
	assertSplit(t, result, []string{"echo $(echo $(whoami))"})
}

func TestSplitShell_SubshellWithOperator(t *testing.T) {
	result := splitShellCommands("echo $(echo a && echo b) && rm -rf /")
	assertSplit(t, result, []string{"echo $(echo a && echo b)", "rm -rf /"})
}

func TestSplitShell_Backtick(t *testing.T) {
	result := splitShellCommands("echo `whoami`")
	assertSplit(t, result, []string{"echo `whoami`"})
}

func TestSplitShell_EmptyInput(t *testing.T) {
	result := splitShellCommands("")
	assertSplit(t, result, nil)
}

func TestSplitShell_OnlyWhitespace(t *testing.T) {
	result := splitShellCommands("   ")
	assertSplit(t, result, nil)
}

func TestSplitShell_EscapedQuote(t *testing.T) {
	result := splitShellCommands(`echo "hello \"world\"" && rm -rf /`)
	assertSplit(t, result, []string{`echo "hello \"world\""`, "rm -rf /"})
}

func assertSplit(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d commands %v, want %d commands %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("cmd[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
