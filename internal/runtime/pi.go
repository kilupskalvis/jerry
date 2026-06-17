package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// PiOptions configures the pi adapter.
type PiOptions struct {
	// PinnedVersion, when non-empty, is required to match `pi --version`
	// before any invocation. Empty disables the preflight.
	PinnedVersion string
	// Binary overrides the executable name (default "pi"); for tests.
	Binary string
}

// Pi runs the pi coding agent CLI as a single-step runtime.
type Pi struct {
	opts PiOptions
}

// NewPi constructs the pi adapter.
func NewPi(opts PiOptions) *Pi {
	if opts.Binary == "" {
		opts.Binary = "pi"
	}
	return &Pi{opts: opts}
}

func (p *Pi) Name() string { return "pi" }

func (p *Pi) Capabilities() Capabilities {
	return Capabilities{
		StructuredOutput: false, // exec provides structured output generically
		CostReporting:    true,  // pi reports per-message usage incl. cost.total
		Permissions:      true,  // tool allowlist (allow patterns; deny not representable)
		Streaming:        false,
	}
}

// Invoke spawns pi, feeds the prompt as a positional arg with stdin closed,
// captures stdout, and parses the JSONL session.
func (p *Pi) Invoke(ctx context.Context, inv InvocationSpec) (Result, error) {
	if err := p.preflight(); err != nil {
		return Result{}, err
	}

	cmd := exec.CommandContext(ctx, p.opts.Binary, buildArgs(inv)...)
	cmd.Dir = inv.Workdir
	cmd.Env = piEnv(inv.Env)
	cmd.Stdin = nil // closed: pi hangs on open stdin in print mode
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("pi exited abnormally: %w (stderr: %s)", err, stderr.String())
	}

	parsed, err := parseSession(stdout.Bytes())
	if err != nil {
		return Result{}, err
	}
	return Result{Text: parsed.Text, Usage: parsed.Usage}, nil
}

// preflight verifies the installed pi version matches the lockfile pin
// (filled in by the version-preflight task).
func (p *Pi) preflight() error { return nil }

// piEnv builds the child environment: the caller's allowlist (API keys)
// plus PATH and HOME so the binary and its config resolve.
func piEnv(allow []string) []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	return append(env, allow...)
}
