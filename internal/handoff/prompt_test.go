package handoff

import (
	"strings"
	"testing"
)

func TestBuildPromptDefaultContext(t *testing.T) {
	ctx := testRunContext()
	ctx.Order = []string{"plan"}

	got, err := BuildPrompt("Do the thing.", nil, ctx)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	for _, want := range []string{
		"<untrusted-trigger>", "Fix the bug", "</untrusted-trigger>",
		"## Previous step: plan", "plan text",
		"Do the thing.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q in:\n%s", want, got)
		}
	}
	if strings.Index(got, "plan text") > strings.Index(got, "Do the thing.") {
		t.Error("instructions must come after context")
	}
}

func TestBuildPromptExplicitContext(t *testing.T) {
	ctx := testRunContext()
	ctx.Order = []string{"plan"}

	got, err := BuildPrompt("Review it.", []string{"diff:plan"}, ctx)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(got, "diff body") {
		t.Error("explicit diff context missing")
	}
	if strings.Contains(got, "plan text") {
		t.Error("explicit context must exclude unselected step outputs")
	}
	if strings.Contains(got, "<untrusted-trigger>") {
		t.Error("trigger not requested — must be absent")
	}
}

func TestBuildPromptUnknownContextEntry(t *testing.T) {
	ctx := testRunContext()
	if _, err := BuildPrompt("x", []string{"steps.ghost"}, ctx); err == nil {
		t.Fatal("want error for unknown context step")
	}
}
