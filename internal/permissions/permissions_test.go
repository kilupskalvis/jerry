package permissions_test

import (
	"testing"

	"github.com/kilupskalvis/jerry/internal/permissions"
)

func TestMerge_DenyUnion(t *testing.T) {
	t.Parallel()
	base := permissions.Permissions{
		Deny: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"rm *"}}},
	}
	overlay := permissions.Permissions{
		Deny: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"curl *"}}},
	}

	merged := base.Merge(overlay)

	rules := merged.DenyFor("bash")
	if len(rules) != 2 {
		t.Fatalf("got %d deny rules for bash, want 2", len(rules))
	}
}

func TestMerge_AllowIntersect_BothPresent(t *testing.T) {
	t.Parallel()
	base := permissions.Permissions{
		Allow: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"go *", "npm *", "make *"}}},
	}
	overlay := permissions.Permissions{
		Allow: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"go *", "npm *"}}},
	}

	merged := base.Merge(overlay)

	rules := merged.AllowFor("bash")
	if len(rules) != 2 {
		t.Fatalf("got %d allow rules for bash, want 2: %v", len(rules), rules)
	}
}

func TestMerge_AllowIntersect_ParentOnly(t *testing.T) {
	t.Parallel()
	base := permissions.Permissions{
		Allow: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"go *"}}},
	}
	overlay := permissions.Permissions{}

	merged := base.Merge(overlay)

	rules := merged.AllowFor("bash")
	if len(rules) != 1 || rules[0] != "go *" {
		t.Errorf("parent allow should be inherited, got %v", rules)
	}
}

func TestMerge_AllowIntersect_ChildOnly(t *testing.T) {
	t.Parallel()
	base := permissions.Permissions{}
	overlay := permissions.Permissions{
		Allow: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"go *"}}},
	}

	merged := base.Merge(overlay)

	rules := merged.AllowFor("bash")
	if len(rules) != 1 || rules[0] != "go *" {
		t.Errorf("child allow should become effective, got %v", rules)
	}
}

func TestMerge_AllowIntersect_NeitherPresent(t *testing.T) {
	t.Parallel()
	merged := permissions.Permissions{}.Merge(permissions.Permissions{})

	rules := merged.AllowFor("bash")
	if rules != nil {
		t.Errorf("no allow restriction expected, got %v", rules)
	}
}

func TestMerge_MultipleLayers(t *testing.T) {
	t.Parallel()
	settings := permissions.Permissions{
		Deny:  []permissions.ToolRule{{Tool: "bash", Patterns: []string{"rm *"}}},
		Allow: []permissions.ToolRule{{Tool: "write_file", Patterns: []string{"src/**", "tests/**", "docs/**"}}},
	}
	local := permissions.Permissions{
		Deny: []permissions.ToolRule{{Tool: "bash", Patterns: []string{"docker *"}}},
	}
	agent := permissions.Permissions{
		Allow: []permissions.ToolRule{{Tool: "write_file", Patterns: []string{"src/**"}}},
	}

	merged := settings.Merge(local).Merge(agent)

	bashDeny := merged.DenyFor("bash")
	if len(bashDeny) != 2 {
		t.Errorf("bash deny should have 2 patterns, got %d", len(bashDeny))
	}

	writeAllow := merged.AllowFor("write_file")
	if len(writeAllow) != 1 || writeAllow[0] != "src/**" {
		t.Errorf("write_file allow should be narrowed to [src/**], got %v", writeAllow)
	}
}
