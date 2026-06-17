package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilupskalvis/jerry/internal/spec"
)

func fakeRuntimeScript(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCustomAdapterArgMode(t *testing.T) {
	fakeRuntimeScript(t, "fakert", `echo '{"result":{"text":"hello from fake"}}'`)

	a := NewCustom(spec.AdapterSpec{
		Name:    "fakert",
		Command: "fakert",
		Prompt:  "arg",
		Parse:   spec.ParseSpec{Text: "result.text"},
	})

	if a.Name() != "fakert" {
		t.Errorf("Name = %q", a.Name())
	}

	res, err := a.Invoke(t.Context(), InvocationSpec{
		Prompt:  "test prompt",
		Workdir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "hello from fake" {
		t.Errorf("Text = %q", res.Text)
	}
}

func TestCustomAdapterStdinMode(t *testing.T) {
	fakeRuntimeScript(t, "stdinrt", `input=$(cat); echo "{\"out\":\"got: $input\"}"`)

	a := NewCustom(spec.AdapterSpec{
		Name:    "stdinrt",
		Command: "stdinrt",
		Prompt:  "stdin",
		Parse:   spec.ParseSpec{Text: "out"},
	})

	res, err := a.Invoke(t.Context(), InvocationSpec{
		Prompt:  "hello",
		Workdir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "got: hello") {
		t.Errorf("Text = %q", res.Text)
	}
}

func TestCustomAdapterCostParsing(t *testing.T) {
	fakeRuntimeScript(t, "costrt", `echo '{"text":"ok","usage":{"cost":0.05,"in":100,"out":50}}'`)

	a := NewCustom(spec.AdapterSpec{
		Name:    "costrt",
		Command: "costrt",
		Prompt:  "arg",
		Parse: spec.ParseSpec{
			Text:         "text",
			Cost:         "usage.cost",
			InputTokens:  "usage.in",
			OutputTokens: "usage.out",
		},
		Capabilities: spec.AdapterCapabilities{CostReporting: true},
	})

	res, err := a.Invoke(t.Context(), InvocationSpec{Prompt: "x", Workdir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if res.Usage.CostUSD != 0.05 {
		t.Errorf("CostUSD = %v", res.Usage.CostUSD)
	}
	if res.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d", res.Usage.InputTokens)
	}
}

func TestCustomAdapterCommandFailure(t *testing.T) {
	fakeRuntimeScript(t, "failrt", `exit 1`)

	a := NewCustom(spec.AdapterSpec{
		Name: "failrt", Command: "failrt", Prompt: "arg",
		Parse: spec.ParseSpec{Text: "text"},
	})

	_, err := a.Invoke(t.Context(), InvocationSpec{Prompt: "x", Workdir: t.TempDir()})
	if err == nil {
		t.Fatal("want error on non-zero exit")
	}
}
