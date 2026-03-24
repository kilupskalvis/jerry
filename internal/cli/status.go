// motif status: quick project overview.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kilupskalvis/motif/internal/state"
)

func newStatusCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show project overview",
		RunE: func(_ *cobra.Command, _ []string) error {
			if app.StateStore == nil || app.Loader == nil {
				fmt.Fprintln(os.Stderr, "motif: error: not in a Motif project (no .motif/ directory found)")
				os.Exit(1)
			}
			return showStatus(app)
		},
	}
}

func showStatus(app *App) error {
	motifDir := app.Loader.MotifDir()

	fmt.Fprintf(os.Stderr, "Motif project: %s\n", motifDir)

	// List pipelines.
	pipelineNames := listFilesWithExt(filepath.Join(motifDir, "pipelines"), ".yaml", ".yml")
	if len(pipelineNames) > 0 {
		fmt.Fprintf(os.Stderr, "Pipelines: %d (%s)\n", len(pipelineNames), strings.Join(pipelineNames, ", "))
	} else {
		fmt.Fprintln(os.Stderr, "Pipelines: none")
	}

	// List agents.
	agentNames := listFilesWithExt(filepath.Join(motifDir, "agents"), ".md")
	if len(agentNames) > 0 {
		fmt.Fprintf(os.Stderr, "Agents: %d (%s)\n", len(agentNames), strings.Join(agentNames, ", "))
	} else {
		fmt.Fprintln(os.Stderr, "Agents: none")
	}

	// Run summary.
	summaries, listErr := app.StateStore.ListRuns()
	if listErr != nil {
		return listErr
	}

	if len(summaries) == 0 {
		fmt.Fprintln(os.Stderr, "Runs: none")
		return nil
	}

	var completed, failed int
	for _, s := range summaries {
		switch s.Status {
		case state.StatusCompleted:
			completed++
		case state.StatusFailed:
			failed++
		}
	}
	fmt.Fprintf(os.Stderr, "Runs: %d total (%d completed, %d failed)\n",
		len(summaries), completed, failed)

	// Last run.
	last := summaries[0]
	status := "✓ " + string(last.Status)
	if last.Status == state.StatusFailed {
		status = "✗ " + string(last.Status)
	}
	ago := time.Since(last.StartedAt).Truncate(time.Second)
	fmt.Fprintf(os.Stderr, "Last run: %s (%s) — %s — %s ago\n",
		last.RunID, last.PipelineName, status, ago)

	return nil
}

func listFilesWithExt(dir string, exts ...string) []string {
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, ext := range exts {
			if strings.HasSuffix(entry.Name(), ext) {
				name := strings.TrimSuffix(entry.Name(), ext)
				names = append(names, name)
				break
			}
		}
	}
	return names
}
