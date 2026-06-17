package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kilupskalvis/jerry/internal/handoff"
	"github.com/kilupskalvis/jerry/internal/spec"
)

// CustomAdapter implements Adapter for YAML-declared community runtimes.
type CustomAdapter struct {
	adapterSpec spec.AdapterSpec
}

// NewCustom constructs a custom adapter from a parsed YAML spec.
func NewCustom(s spec.AdapterSpec) *CustomAdapter {
	return &CustomAdapter{adapterSpec: s}
}

func (c *CustomAdapter) Name() string { return c.adapterSpec.Name }

func (c *CustomAdapter) Capabilities() Capabilities {
	return Capabilities{
		StructuredOutput: c.adapterSpec.Capabilities.StructuredOutput,
		CostReporting:    c.adapterSpec.Capabilities.CostReporting,
		Permissions:      c.adapterSpec.Capabilities.Permissions,
	}
}

// Invoke spawns the command, delivers the prompt, captures JSON stdout,
// and extracts fields via dot-path expressions.
func (c *CustomAdapter) Invoke(ctx context.Context, inv InvocationSpec) (Result, error) {
	argv := strings.Fields(c.adapterSpec.Command)
	if len(argv) == 0 {
		return Result{}, fmt.Errorf("custom adapter %q: empty command", c.adapterSpec.Name)
	}

	var stdinData string
	switch {
	case c.adapterSpec.Prompt == "arg":
		argv = append(argv, inv.Prompt)
	case c.adapterSpec.Prompt == "stdin":
		stdinData = inv.Prompt
	case strings.HasPrefix(c.adapterSpec.Prompt, "file:"):
		flag := strings.TrimPrefix(c.adapterSpec.Prompt, "file:")
		tmp, err := os.CreateTemp("", "jerry-prompt-*.md")
		if err != nil {
			return Result{}, fmt.Errorf("creating temp prompt file: %w", err)
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.WriteString(inv.Prompt); err != nil {
			tmp.Close()
			return Result{}, err
		}
		tmp.Close()
		argv = append(argv, flag, tmp.Name())
	default:
		return Result{}, fmt.Errorf("custom adapter %q: unknown prompt mode %q (use arg, stdin, or file:<flag>)",
			c.adapterSpec.Name, c.adapterSpec.Prompt)
	}

	if inv.Model != "" {
		argv = append(argv, "--model", inv.Model)
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = inv.Workdir
	cmd.Env = customEnv(inv.Env)
	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("custom adapter %q exited abnormally: %w (stderr: %s)",
			c.adapterSpec.Name, err, stderr.String())
	}

	return c.parseOutput(stdout.Bytes())
}

func (c *CustomAdapter) parseOutput(data []byte) (Result, error) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("custom adapter %q: stdout is not valid JSON: %w", c.adapterSpec.Name, err)
	}

	text, err := handoff.PathLookup(doc, c.adapterSpec.Parse.Text)
	if err != nil {
		return Result{}, fmt.Errorf("custom adapter %q: extracting text via %q: %w",
			c.adapterSpec.Name, c.adapterSpec.Parse.Text, err)
	}

	res := Result{Text: text}
	p := c.adapterSpec.Parse

	if p.Cost != "" || p.InputTokens != "" || p.OutputTokens != "" {
		u := &Usage{}
		if p.Cost != "" {
			if v, lookErr := handoff.PathLookup(doc, p.Cost); lookErr == nil {
				u.CostUSD, _ = strconv.ParseFloat(v, 64)
			}
		}
		if p.InputTokens != "" {
			if v, lookErr := handoff.PathLookup(doc, p.InputTokens); lookErr == nil {
				u.InputTokens, _ = strconv.ParseInt(v, 10, 64)
			}
		}
		if p.OutputTokens != "" {
			if v, lookErr := handoff.PathLookup(doc, p.OutputTokens); lookErr == nil {
				u.OutputTokens, _ = strconv.ParseInt(v, 10, 64)
			}
		}
		res.Usage = u
	}

	return res, nil
}

func customEnv(allow []string) []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	return append(env, allow...)
}
