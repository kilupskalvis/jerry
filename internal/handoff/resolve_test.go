package handoff

import (
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

func testRunContext() *RunContext {
	return &RunContext{
		Trigger: trigger.TriggerData{
			Type: "pull_request", Source: "github", Intent: "Fix the bug",
			RawPayload: map[string]any{"pull_request": map[string]any{"number": float64(7)}},
		},
		Run: RunMeta{ID: "r1", Cost: 1.2345, Tokens: 999},
		Steps: map[string]StepRecord{
			"plan": {
				Name: "plan", Output: "plan text",
				Outputs:  map[string]any{"approach": "small steps", "files": []any{"a.go", "b.go"}},
				Diff:     "diff body",
				DiffStat: "2 files changed",
			},
		},
	}
}

func TestResolve(t *testing.T) {
	ctx := testRunContext()
	cases := []struct{ name, in, want string }{
		{"trigger field", "X ${{ trigger.intent }} Y", "X Fix the bug Y"},
		{"trigger raw", "#${{ trigger.raw.pull_request.number }}", "#7"},
		{"step output", "${{ steps.plan.output }}", "plan text"},
		{"step outputs string", "${{ steps.plan.outputs.approach }}", "small steps"},
		{"step outputs list is json", "${{ steps.plan.outputs.files }}", `["a.go","b.go"]`},
		{"diff", "${{ steps.plan.diff }}", "diff body"},
		{"diff stat", "${{ steps.plan.diff_stat }}", "2 files changed"},
		{"run id", "${{ run.id }}", "r1"},
		{"run cost", "${{ run.cost }}", "1.2345"},
		{"run tokens", "${{ run.tokens }}", "999"},
		{"no refs", "plain text", "plain text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.in, ctx)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveErrors(t *testing.T) {
	ctx := testRunContext()
	cases := []struct{ name, in, wantSubstr string }{
		{"unknown step", "${{ steps.ghost.output }}", `step "ghost" has no record`},
		{"unknown output key", "${{ steps.plan.outputs.nope }}", `no output "nope"`},
		{"bad raw path", "${{ trigger.raw.missing.x }}", "not found"},
		{"syntax", "${{ steps.plan }}", "expected steps.<name>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resolve(tc.in, ctx)
			if err == nil || !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantSubstr)
			}
		})
	}
}
