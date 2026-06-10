package spec

import "testing"

func TestPermissionSetMergeDeny(t *testing.T) {
	p := PermissionSet{Allow: []string{"read"}, Deny: []string{"bash(rm:*)"}}
	merged := p.MergeDeny([]string{"read(.env)", "bash(rm:*)"})
	if len(merged.Deny) != 2 {
		t.Errorf("Deny = %v, want deduped 2 entries", merged.Deny)
	}
	if len(merged.Allow) != 1 || merged.Allow[0] != "read" {
		t.Errorf("Allow mutated: %v", merged.Allow)
	}
	if len(p.Deny) != 1 {
		t.Error("receiver mutated")
	}
}

func TestStepKind(t *testing.T) {
	cases := []struct {
		name string
		step Step
		want StepKind
	}{
		{"agent", Step{Prompt: "p.md"}, KindAgent},
		{"shell", Step{Run: "go test ./..."}, KindShell},
		{"ci", Step{CI: "post_pr_comment"}, KindCI},
		{"none", Step{}, KindInvalid},
		{"two set", Step{Prompt: "p.md", Run: "ls"}, KindInvalid},
		{"all set", Step{Prompt: "p", Run: "r", CI: "c"}, KindInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.step.Kind(); got != tc.want {
				t.Errorf("Kind() = %v, want %v", got, tc.want)
			}
		})
	}
}
