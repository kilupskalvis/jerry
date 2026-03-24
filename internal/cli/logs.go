// motif logs: view execution history and run details.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	motifErrors "github.com/kilupskalvis/motif/internal/errors"
	"github.com/kilupskalvis/motif/internal/state"
)

func newLogsCmd(app *App) *cobra.Command {
	var (
		stepFilter string
		showTools  bool
		showLLM    bool
		showJSON   bool
		showLast   bool
	)

	cmd := &cobra.Command{
		Use:   "logs [run-id]",
		Short: "View execution logs",
		Long:  "List recent runs or show detailed logs for a specific run.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if app.StateStore == nil {
				return motifErrors.New(motifErrors.CodeMotifDirNotFound,
					"not in a Motif project (no .motif/ directory found) — run 'motif init' to initialize")
			}

			if showLast {
				return showLastRun(app)
			}

			if len(args) == 0 {
				return listRuns(app)
			}

			return showRunDetail(app, args[0], stepFilter, showTools, showLLM, showJSON)
		},
	}

	cmd.Flags().StringVar(&stepFilter, "step", "", "Show details for a specific step")
	cmd.Flags().BoolVar(&showTools, "tools", false, "Show all tool calls")
	cmd.Flags().BoolVar(&showLLM, "llm", false, "Show all LLM calls")
	cmd.Flags().BoolVar(&showJSON, "json", false, "Output raw JSONL")
	cmd.Flags().BoolVar(&showLast, "last", false, "Show the most recent run")

	return cmd
}

func listRuns(app *App) error {
	summaries, listErr := app.StateStore.ListRuns()
	if listErr != nil {
		return listErr
	}

	if len(summaries) == 0 {
		fmt.Fprintln(os.Stderr, "No runs found.")
		return nil
	}

	fmt.Fprintln(os.Stderr, "Recent runs:")
	limit := 20
	if len(summaries) < limit {
		limit = len(summaries)
	}
	for _, s := range summaries[:limit] {
		status := "✓ " + string(s.Status)
		if s.Status == state.StatusFailed {
			status = "✗ " + string(s.Status)
		}
		ago := time.Since(s.StartedAt).Truncate(time.Second)
		fmt.Fprintf(os.Stderr, "  %-18s %-16s %s  %d steps  %s ago\n",
			s.RunID, s.PipelineName, status, s.StepCount, ago)
	}
	return nil
}

func showLastRun(app *App) error {
	summaries, listErr := app.StateStore.ListRuns()
	if listErr != nil {
		return listErr
	}
	if len(summaries) == 0 {
		fmt.Fprintln(os.Stderr, "No runs found.")
		return nil
	}
	return showRunDetail(app, summaries[0].RunID, "", false, false, false)
}

func showRunDetail(app *App, runID, stepFilter string, showTools, showLLM, showJSON bool) error {
	runState, loadErr := app.StateStore.LoadRun(runID)
	if loadErr != nil {
		return loadErr
	}

	// Read structured log entries.
	runDir := app.StateStore.RunDir(runID)
	entries, _ := state.ReadLogEntries(runDir)

	if showJSON {
		for _, entry := range entries {
			line, _ := json.Marshal(entry)
			fmt.Fprintln(os.Stdout, string(line))
		}
		return nil
	}

	if stepFilter != "" {
		return showStepDetail(runState, entries, stepFilter)
	}

	if showTools {
		return showToolCalls(entries)
	}

	if showLLM {
		return showLLMCalls(entries)
	}

	// Default: run overview.
	return showRunOverview(runState, entries)
}

func showRunOverview(runState *state.RunState, _ []state.LogEntry) error {
	fmt.Fprintf(os.Stderr, "Run: %s\n", runState.RunID)
	fmt.Fprintf(os.Stderr, "Pipeline: %s\n", runState.PipelineName)
	fmt.Fprintf(os.Stderr, "Status: %s\n", runState.Status)
	fmt.Fprintf(os.Stderr, "Started: %s\n", runState.StartedAt.Format("2006-01-02 15:04:05"))
	if runState.CompletedAt != nil {
		duration := runState.CompletedAt.Sub(runState.StartedAt)
		fmt.Fprintf(os.Stderr, "Duration: %s\n", formatLogDuration(duration))
	}

	fmt.Fprintln(os.Stderr, "\nSteps:")
	for _, r := range runState.StepResults {
		var symbol string
		switch r.Status {
		case state.StepFailed:
			symbol = "✗"
		case state.StepSkipped:
			symbol = "⊘"
		default:
			symbol = "✓"
		}
		duration := time.Duration(r.DurationMs) * time.Millisecond
		fmt.Fprintf(os.Stderr, "  %s %-16s %-8s %s\n",
			symbol, r.Name, r.Type, formatLogDuration(duration))
	}
	return nil
}

func showStepDetail(_ *state.RunState, entries []state.LogEntry, stepName string) error {
	fmt.Fprintf(os.Stderr, "Step: %s\n\n", stepName)

	toolCount := 0
	for _, entry := range entries {
		if entry.Step != stepName && entry.Step != "" {
			continue
		}

		switch entry.Type {
		case state.LogToolCall:
			var data state.ToolCallData
			if json.Unmarshal(entry.Data, &data) == nil {
				toolCount++
				status := "✓"
				if !data.Success {
					status = "✗"
				}
				fmt.Fprintf(os.Stderr, "  %3d. %s %-16s %dms\n",
					toolCount, status, data.Tool, data.DurationMs)
			}
		case state.LogLLMCall:
			var data state.LLMCallData
			if json.Unmarshal(entry.Data, &data) == nil {
				tools := ""
				if len(data.ToolCallsRequested) > 0 {
					tools = " → " + strings.Join(data.ToolCallsRequested, ", ")
				}
				fmt.Fprintf(os.Stderr, "  [llm iter %d] %dk tokens, %dms%s\n",
					data.Iteration,
					(data.TokensInput+data.TokensOutput)/1000,
					data.DurationMs, tools)
			}
		}
	}

	if toolCount == 0 {
		fmt.Fprintln(os.Stderr, "  (no tool calls recorded)")
	}
	return nil
}

func showToolCalls(entries []state.LogEntry) error {
	fmt.Fprintln(os.Stderr, "Tool calls:")
	currentStep := ""
	count := 0
	for _, entry := range entries {
		if entry.Type != state.LogToolCall {
			continue
		}
		if entry.Step != currentStep {
			currentStep = entry.Step
			fmt.Fprintf(os.Stderr, "\n  Step: %s\n", currentStep)
		}
		var data state.ToolCallData
		if json.Unmarshal(entry.Data, &data) == nil {
			count++
			fmt.Fprintf(os.Stderr, "    %-16s %dms\n", data.Tool, data.DurationMs)
		}
	}
	if count == 0 {
		fmt.Fprintln(os.Stderr, "  (no tool calls recorded)")
	}
	return nil
}

func showLLMCalls(entries []state.LogEntry) error {
	fmt.Fprintln(os.Stderr, "LLM calls:")
	count := 0
	for _, entry := range entries {
		if entry.Type != state.LogLLMCall {
			continue
		}
		var data state.LLMCallData
		if json.Unmarshal(entry.Data, &data) == nil {
			count++
			fmt.Fprintf(os.Stderr, "  [%s iter %d] model=%s in=%d out=%d %dms stop=%s\n",
				entry.Step, data.Iteration, data.Model,
				data.TokensInput, data.TokensOutput,
				data.DurationMs, data.StopReason)
		}
	}
	if count == 0 {
		fmt.Fprintln(os.Stderr, "  (no LLM calls recorded)")
	}
	return nil
}

func formatLogDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
