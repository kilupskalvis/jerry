package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestRegistryLookup(t *testing.T) {
	fake := NewFake("pi")
	r := NewRegistry(fake)

	got, err := r.Lookup("pi")
	if err != nil || got != fake {
		t.Fatalf("Lookup(pi) = %v, %v", got, err)
	}
}

func TestRegistryLookupSuggests(t *testing.T) {
	r := NewRegistry(NewFake("pi"), NewFake("claude-code"))
	_, err := r.Lookup("claude-cod")
	if err == nil {
		t.Fatal("want error for unknown runtime")
	}
	if !strings.Contains(err.Error(), `did you mean "claude-code"`) {
		t.Errorf("error = %v", err)
	}
}

func TestFakeRecordsAndScripts(t *testing.T) {
	fake := NewFake("pi")
	fake.Script(Result{Text: "done", Usage: &Usage{CostUSD: 0.10}})

	inv := InvocationSpec{Prompt: "hello", Model: "m1"}
	res, err := fake.Invoke(context.Background(), inv)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.Text != "done" || res.Usage.CostUSD != 0.10 {
		t.Errorf("Result = %+v", res)
	}
	if len(fake.Invocations) != 1 || fake.Invocations[0].Prompt != "hello" {
		t.Errorf("Invocations = %+v", fake.Invocations)
	}
}

func TestFakeErrScript(t *testing.T) {
	fake := NewFake("pi")
	fake.ScriptErr(context.DeadlineExceeded)
	if _, err := fake.Invoke(context.Background(), InvocationSpec{}); err == nil {
		t.Fatal("want scripted error")
	}
}
