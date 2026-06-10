package spec

import "testing"

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
