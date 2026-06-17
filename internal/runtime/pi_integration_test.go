//go:build integration

package runtime

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/spec"
)

func TestPiInvokeReal(t *testing.T) {
	if _, err := exec.LookPath("pi"); err != nil {
		t.Skip("pi not installed")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	pi := NewPi(PiOptions{})
	res, err := pi.Invoke(context.Background(), InvocationSpec{
		Prompt:      "Reply with exactly the text: integration ok",
		Model:       "claude-haiku-4-5",
		Env:         []string{"ANTHROPIC_API_KEY=" + os.Getenv("ANTHROPIC_API_KEY")},
		Permissions: spec.PermissionSet{},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !strings.Contains(strings.ToLower(res.Text), "integration ok") {
		t.Errorf("unexpected text: %q", res.Text)
	}
	if res.Usage == nil || res.Usage.OutputTokens == 0 {
		t.Errorf("expected nonzero usage, got %+v", res.Usage)
	}
}
