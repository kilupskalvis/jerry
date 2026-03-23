package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kilupskalvis/motif/internal/agent"
	"github.com/kilupskalvis/motif/internal/pipeline"
)

func TestAgentExecutor_CanExecute_True(t *testing.T) {
	exec := agent.NewExecutor()
	step := pipeline.Step{Name: "gen", Agent: "./agents/generate.md"}

	if !exec.CanExecute(step) {
		t.Error("should return true for steps with Agent set")
	}
}

func TestAgentExecutor_CanExecute_False(t *testing.T) {
	exec := agent.NewExecutor()
	step := pipeline.Step{Name: "test", Script: "echo hi"}

	if exec.CanExecute(step) {
		t.Error("should return false for steps with Script set")
	}
}

func TestAgentExecutor_CanExecute_Empty(t *testing.T) {
	exec := agent.NewExecutor()
	step := pipeline.Step{Name: "empty"}

	if exec.CanExecute(step) {
		t.Error("should return false for empty steps")
	}
}

func TestAgentExecutor_Execute_ReturnsSkip(t *testing.T) {
	exec := agent.NewExecutor()
	step := pipeline.Step{Name: "gen", Agent: "./agents/generate.md"}

	_, err := exec.Execute(context.Background(), step, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var skipErr pipeline.ErrSkipStep
	if !errors.As(err, &skipErr) {
		t.Fatalf("error should be ErrSkipStep, got %T: %v", err, err)
	}

	if skipErr.Reason == "" {
		t.Error("ErrSkipStep.Reason should not be empty")
	}
}
