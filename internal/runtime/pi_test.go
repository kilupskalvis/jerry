package runtime

import (
	"slices"
	"testing"

	"github.com/kilupskalvis/jerry/internal/spec"
)

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
