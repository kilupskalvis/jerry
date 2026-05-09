// jerry logs: view project overview, execution history, and run details.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/run"
)

// @lattice:flow logs
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
		Short: "View project overview and execution logs",
		Long:  "Show project overview and recent runs, or detailed logs for a specific run.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if app.StateStore == nil {
				return jerrerr.New(jerrerr.CodeJerryDirNotFound,
					"not in a Jerry project (no .jerry/ directory found) — run 'jerry init' to initialize")
			}

			if showLast {
				return showLastRun(app)
			}

			if len(args) == 0 {
				return showOverview(app)
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

func showOverview(app *App) error {
	if app.Loader != nil {
		jerryDir := app.Loader.JerryDir()
		fmt.Fprintf(os.Stderr, "Jerry project: %s\n", jerryDir)

		workflowNames := app.Loader.ListWorkflows()
		if len(workflowNames) > 0 {
			fmt.Fprintf(os.Stderr, "Workflows: %d (%s)\n", len(workflowNames), strings.Join(workflowNames, ", "))
		} else {
			fmt.Fprintln(os.Stderr, "Workflows: none")
		}

		fmt.Fprintln(os.Stderr)
	}

	summaries, listErr := app.StateStore.ListRuns()
	if listErr != nil {
		return listErr
	}

	if len(summaries) == 0 {
		fmt.Fprintln(os.Stderr, "No runs found.")
		return nil
	}

	var completed, failed int
	for _, s := range summaries {
		switch s.Status {
		case run.StatusCompleted:
			completed++
		case run.StatusFailed:
			failed++
		}
	}
	fmt.Fprintf(os.Stderr, "Runs: %d total (%d completed, %d failed)\n\n", len(summaries), completed, failed)

	fmt.Fprintln(os.Stderr, "Recent runs:")
	limit := 20
	if len(summaries) < limit {
		limit = len(summaries)
	}
	for _, s := range summaries[:limit] {
		status := "✓ " + string(s.Status)
		if s.Status == run.StatusFailed {
			status = "✗ " + string(s.Status)
		}
		ago := time.Since(s.StartedAt).Truncate(time.Second)
		fmt.Fprintf(os.Stderr, "  %-18s %-16s %s  %d steps  %s ago\n",
			s.RunID, s.WorkflowName, status, s.StepCount, ago)
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

	runDir := app.StateStore.RunDir(runID)
	entries, _ := run.ReadLogEntries(runDir)

	if showJSON {
		for _, entry := range entries {
			line, _ := json.Marshal(entry)
			fmt.Fprintln(os.Stdout, string(line))
		}
		return nil
	}

	if stepFilter != "" {
		return showStepDetail(entries, stepFilter)
	}

	if showTools {
		return showToolCalls(entries)
	}

	if showLLM {
		return showLLMCalls(entries)
	}

	return showRunOverview(runState, entries)
}

// showRunOverview prints the run summary: header, token totals, per-step stats,
// files changed, and errors. All tool-specific knowledge comes from the stored
// Summary field — this function is completely tool-agnostic.
func showRunOverview(runState *run.RunState, entries []run.LogEntry) error {
	fmt.Fprintf(os.Stderr, "Run: %s\n", runState.RunID)
	fmt.Fprintf(os.Stderr, "Workflow: %s\n", runState.WorkflowName)
	fmt.Fprintf(os.Stderr, "Status: %s\n", runState.Status)
	fmt.Fprintf(os.Stderr, "Started: %s\n", runState.StartedAt.Format("2006-01-02 15:04:05"))
	if runState.CompletedAt != nil {
		duration := runState.CompletedAt.Sub(runState.StartedAt)
		fmt.Fprintf(os.Stderr, "Duration: %s\n", formatLogDuration(duration))
	}

	var totalInput, totalOutput int
	for _, entry := range entries {
		if entry.Type != run.LogLLMCall {
			continue
		}
		var data run.LLMCallData
		if json.Unmarshal(entry.Data, &data) == nil {
			totalInput += data.TokensInput
			totalOutput += data.TokensOutput
		}
	}
	if totalInput > 0 || totalOutput > 0 {
		fmt.Fprintf(os.Stderr, "Tokens: %dk input, %dk output (%dk total)\n",
			totalInput/1000, totalOutput/1000, (totalInput+totalOutput)/1000)
	}

	stepStats := aggregateByStep(entries)

	fmt.Fprintln(os.Stderr, "\nSteps:")
	for _, r := range runState.StepResults {
		symbol := "✓"
		switch r.Status {
		case run.StepFailed:
			symbol = "✗"
		case run.StepSkipped:
			symbol = "⊘"
		}
		duration := time.Duration(r.DurationMs) * time.Millisecond

		extra := ""
		if s, ok := stepStats[r.Name]; ok {
			parts := []string{}
			if s.llmCalls > 0 {
				parts = append(parts, fmt.Sprintf("%d LLM calls", s.llmCalls))
			}
			if s.toolCalls > 0 {
				parts = append(parts, fmt.Sprintf("%d tool calls", s.toolCalls))
			}
			if s.tokensIn > 0 {
				parts = append(parts, fmt.Sprintf("%dk tokens", (s.tokensIn+s.tokensOut)/1000))
			}
			if len(parts) > 0 {
				extra = "  " + strings.Join(parts, ", ")
			}
		}

		fmt.Fprintf(os.Stderr, "  %s %-16s %-8s %s%s\n",
			symbol, r.Name, r.Type, formatLogDuration(duration), extra)
	}

	// Files changed — detected by tool calls that have both path and content args.
	filesWritten := extractWrittenFiles(entries)
	if len(filesWritten) > 0 {
		fmt.Fprintln(os.Stderr, "\nFiles changed:")
		for _, f := range filesWritten {
			fmt.Fprintf(os.Stderr, "  ~ %s\n", f)
		}
	}

	for _, r := range runState.StepResults {
		if r.Status == run.StepFailed && r.Error != nil {
			fmt.Fprintf(os.Stderr, "\nError in step %q: %s\n", r.Name, r.Error.Message)
		}
	}

	return nil
}

// showStepDetail prints every log entry for a step. Tool call summaries come
// from the stored Summary field — no tool-specific interpretation here.
func showStepDetail(entries []run.LogEntry, stepName string) error {
	s := aggregateByStep(entries)[stepName]
	if s != nil {
		parts := []string{}
		if s.llmCalls > 0 {
			parts = append(parts, fmt.Sprintf("%d LLM calls", s.llmCalls))
		}
		if s.toolCalls > 0 {
			parts = append(parts, fmt.Sprintf("%d tool calls", s.toolCalls))
		}
		if s.tokensIn > 0 {
			parts = append(parts, fmt.Sprintf("%dk tokens", (s.tokensIn+s.tokensOut)/1000))
		}
		if len(parts) > 0 {
			fmt.Fprintf(os.Stderr, "Step: %s (%s)\n\n", stepName, strings.Join(parts, ", "))
		} else {
			fmt.Fprintf(os.Stderr, "Step: %s\n\n", stepName)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Step: %s\n\n", stepName)
	}

	hasEntries := false
	for _, entry := range entries {
		if entry.Step != stepName {
			continue
		}

		switch entry.Type {
		case run.LogToolCall:
			var data run.ToolCallData
			if json.Unmarshal(entry.Data, &data) == nil {
				hasEntries = true
				status := "✓"
				if !data.Success {
					status = "✗"
				}
				detail := data.Summary
				if detail != "" && data.ResultSizeBytes > 0 {
					detail += fmt.Sprintf(" → %s", formatBytes(data.ResultSizeBytes))
				}
				fmt.Fprintf(os.Stderr, "  iter %d: %s %-20s %s\n",
					data.Iteration, status, data.Tool, detail)
			}
		case run.LogLLMCall:
			var data run.LLMCallData
			if json.Unmarshal(entry.Data, &data) == nil {
				hasEntries = true
				tools := ""
				if len(data.ToolCallsRequested) > 0 {
					tools = " → " + strings.Join(data.ToolCallsRequested, ", ")
				}
				fmt.Fprintf(os.Stderr, "  [llm iter %d] %d+%d tokens, %s%s\n",
					data.Iteration,
					data.TokensInput, data.TokensOutput,
					formatLogDuration(time.Duration(data.DurationMs)*time.Millisecond),
					tools)
			}
		}
	}

	if !hasEntries {
		fmt.Fprintln(os.Stderr, "  (no entries recorded for this step)")
	}
	return nil
}

func showToolCalls(entries []run.LogEntry) error {
	fmt.Fprintln(os.Stderr, "Tool calls:")
	currentStep := ""
	count := 0
	for _, entry := range entries {
		if entry.Type != run.LogToolCall {
			continue
		}
		if entry.Step != currentStep {
			currentStep = entry.Step
			fmt.Fprintf(os.Stderr, "\n  Step: %s\n", currentStep)
		}
		var data run.ToolCallData
		if json.Unmarshal(entry.Data, &data) == nil {
			count++
			fmt.Fprintf(os.Stderr, "    %-16s %s  %dms\n", data.Tool, data.Summary, data.DurationMs)
		}
	}
	if count == 0 {
		fmt.Fprintln(os.Stderr, "  (no tool calls recorded)")
	}
	return nil
}

func showLLMCalls(entries []run.LogEntry) error {
	fmt.Fprintln(os.Stderr, "LLM calls:")
	count := 0
	for _, entry := range entries {
		if entry.Type != run.LogLLMCall {
			continue
		}
		var data run.LLMCallData
		if json.Unmarshal(entry.Data, &data) == nil {
			count++
			fmt.Fprintf(os.Stderr, "  [%s iter %d] model=%s in=%d out=%d %s stop=%s\n",
				entry.Step, data.Iteration, data.Model,
				data.TokensInput, data.TokensOutput,
				formatLogDuration(time.Duration(data.DurationMs)*time.Millisecond),
				data.StopReason)
		}
	}
	if count == 0 {
		fmt.Fprintln(os.Stderr, "  (no LLM calls recorded)")
	}
	return nil
}

// extractWrittenFiles collects unique file paths from tool calls that wrote
// files. Detected generically by the presence of both "path" and "content"
// in the arguments — no tool name check needed.
func extractWrittenFiles(entries []run.LogEntry) []string {
	seen := map[string]struct{}{}
	var files []string
	for _, entry := range entries {
		if entry.Type != run.LogToolCall {
			continue
		}
		var data run.ToolCallData
		if json.Unmarshal(entry.Data, &data) != nil || !data.Success {
			continue
		}
		path, hasPath := data.Arguments["path"].(string)
		_, hasContent := data.Arguments["content"]
		if _, dup := seen[path]; hasPath && hasContent && !dup {
			seen[path] = struct{}{}
			files = append(files, path)
		}
	}
	return files
}

type stepStat struct {
	llmCalls  int
	toolCalls int
	tokensIn  int
	tokensOut int
}

func aggregateByStep(entries []run.LogEntry) map[string]*stepStat {
	stats := map[string]*stepStat{}
	for _, entry := range entries {
		if entry.Step == "" {
			continue
		}
		if _, ok := stats[entry.Step]; !ok {
			stats[entry.Step] = &stepStat{}
		}
		s := stats[entry.Step]
		switch entry.Type {
		case run.LogLLMCall:
			var data run.LLMCallData
			if json.Unmarshal(entry.Data, &data) == nil {
				s.llmCalls++
				s.tokensIn += data.TokensInput
				s.tokensOut += data.TokensOutput
			}
		case run.LogToolCall:
			s.toolCalls++
		}
	}
	return stats
}

func formatBytes(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	return fmt.Sprintf("%.1fKB", float64(b)/1024)
}

func formatLogDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
