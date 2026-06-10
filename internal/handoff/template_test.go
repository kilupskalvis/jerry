package handoff

import (
	"strings"
	"testing"
)

func TestExtractRefs(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantKind RefKind
		wantStep string
		wantKey  string
	}{
		{"trigger intent", "x ${{ trigger.intent }} y", RefTrigger, "", "intent"},
		{"trigger raw path", "${{ trigger.raw.pull_request.number }}", RefTriggerRaw, "", "pull_request.number"},
		{"step output", "${{ steps.plan.output }}", RefStepOutput, "plan", ""},
		{"step outputs key", "${{ steps.plan.outputs.approach }}", RefStepOutputs, "plan", "approach"},
		{"step diff", "${{ steps.implement.diff }}", RefStepDiff, "implement", ""},
		{"step diff stat", "${{ steps.implement.diff_stat }}", RefStepDiffStat, "implement", ""},
		{"run meta", "${{ run.cost }}", RefRun, "", "cost"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refs, err := ExtractRefs(tc.text)
			if err != nil {
				t.Fatalf("ExtractRefs: %v", err)
			}
			if len(refs) != 1 {
				t.Fatalf("len(refs) = %d, want 1", len(refs))
			}
			r := refs[0]
			if r.Kind != tc.wantKind || r.Step != tc.wantStep || r.Key != tc.wantKey {
				t.Errorf("got %+v, want kind=%v step=%q key=%q", r, tc.wantKind, tc.wantStep, tc.wantKey)
			}
		})
	}
}

func TestExtractRefsMultipleAndNone(t *testing.T) {
	refs, err := ExtractRefs("a ${{ trigger.intent }} b ${{ run.id }} c")
	if err != nil || len(refs) != 2 {
		t.Fatalf("refs=%v err=%v, want 2 refs", refs, err)
	}
	refs, err = ExtractRefs("no templates here, even with $ and { braces }")
	if err != nil || len(refs) != 0 {
		t.Fatalf("refs=%v err=%v, want 0 refs", refs, err)
	}
}

func TestExtractRefsErrors(t *testing.T) {
	cases := []struct{ name, text, wantSubstr string }{
		{"unterminated", "x ${{ trigger.intent", "unterminated"},
		{"empty", "${{ }}", "empty template"},
		{"unknown root", "${{ pipeline.intent }}", `unknown reference root "pipeline"`},
		{"bad trigger field", "${{ trigger.body }}", `unknown trigger field "body"`},
		{"bad run field", "${{ run.speed }}", `unknown run field "speed"`},
		{"steps missing part", "${{ steps.plan }}", "expected steps.<name>.output"},
		{"bad steps attr", "${{ steps.plan.stdout }}", `unknown step attribute "stdout"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExtractRefs(tc.text)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not contain %q", err, tc.wantSubstr)
			}
		})
	}
}
