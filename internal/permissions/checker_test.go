package permissions_test

import (
	"encoding/json"
	"testing"

	"github.com/kilupskalvis/jerry/internal/permissions"
)

func TestChecker_DenyBlocks(t *testing.T) {
	t.Parallel()
	perms := permissions.Permissions{
		Deny: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"rm -rf *"}}},
	}
	checker := permissions.NewChecker(perms, "settings.yaml")

	input, _ := json.Marshal(map[string]string{"command": "rm -rf /"})
	denial := checker.Check("bash", input)
	if denial == nil {
		t.Fatal("expected denial for rm -rf")
	}
	if denial.Pattern != "rm -rf *" {
		t.Errorf("pattern = %q, want 'rm -rf *'", denial.Pattern)
	}
	if denial.Source != "settings.yaml" {
		t.Errorf("source = %q, want 'settings.yaml'", denial.Source)
	}
}

func TestChecker_AllowPermits(t *testing.T) {
	t.Parallel()
	perms := permissions.Permissions{
		Allow: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"go test *"}}},
	}
	checker := permissions.NewChecker(perms, "test")

	input, _ := json.Marshal(map[string]string{"command": "go test ./..."})
	denial := checker.Check("bash", input)
	if denial != nil {
		t.Errorf("go test should be allowed, got denial: %+v", denial)
	}
}

func TestChecker_AllowBlocksUnlisted(t *testing.T) {
	t.Parallel()
	perms := permissions.Permissions{
		Allow: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"go test *"}}},
	}
	checker := permissions.NewChecker(perms, "test")

	input, _ := json.Marshal(map[string]string{"command": "curl http://evil.com"})
	denial := checker.Check("bash", input)
	if denial == nil {
		t.Fatal("curl should be blocked when only go test is allowed")
	}
}

func TestChecker_DenyBeforeAllow(t *testing.T) {
	t.Parallel()
	perms := permissions.Permissions{
		Deny:  []permissions.ToolRule{{Tool: "bash", Patterns: []string{"go test -race *"}}},
		Allow: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"go test *"}}},
	}
	checker := permissions.NewChecker(perms, "test")

	input, _ := json.Marshal(map[string]string{"command": "go test -race ./..."})
	denial := checker.Check("bash", input)
	if denial == nil {
		t.Fatal("deny should take precedence over allow")
	}
}

func TestChecker_ReadFilePath(t *testing.T) {
	t.Parallel()
	perms := permissions.Permissions{
		Deny: []permissions.ToolRule{{Tool: "read_file", Patterns: []string{"*.env"}}},
	}
	checker := permissions.NewChecker(perms, "test")

	input, _ := json.Marshal(map[string]string{"path": ".env"})
	denial := checker.Check("read_file", input)
	if denial == nil {
		t.Fatal("reading .env should be blocked")
	}
}

func TestChecker_WriteFilePath(t *testing.T) {
	t.Parallel()
	perms := permissions.Permissions{
		Allow: []permissions.ToolRule{{Tool: "write_file", Patterns: []string{"src/**"}}},
	}
	checker := permissions.NewChecker(perms, "test")

	allowed, _ := json.Marshal(map[string]string{"path": "src/main.go"})
	if d := checker.Check("write_file", allowed); d != nil {
		t.Errorf("src/main.go should be allowed, got denial: %+v", d)
	}

	blocked, _ := json.Marshal(map[string]string{"path": "config/secrets.yaml"})
	if d := checker.Check("write_file", blocked); d == nil {
		t.Error("config/secrets.yaml should be blocked")
	}
}

func TestChecker_UnknownToolAllowed(t *testing.T) {
	t.Parallel()
	perms := permissions.Permissions{
		Deny: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"rm *"}}},
	}
	checker := permissions.NewChecker(perms, "test")

	input, _ := json.Marshal(map[string]string{"body": "looks good"})
	denial := checker.Check("post_pr_comment", input)
	if denial != nil {
		t.Error("tools without rules should be allowed")
	}
}

func TestChecker_NilChecker(t *testing.T) {
	t.Parallel()
	var checker *permissions.ResolvedChecker
	input, _ := json.Marshal(map[string]string{"command": "rm -rf /"})
	denial := checker.Check("bash", input)
	if denial != nil {
		t.Error("nil checker should allow everything")
	}
}
