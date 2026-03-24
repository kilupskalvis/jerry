// glob tool: finds files matching glob patterns including ** for recursive matching.

package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/kilupskalvis/motif/internal/llm"
)

// MaxGlobResults is the maximum file paths returned from a single glob.
const MaxGlobResults = 200

// NewGlobTool creates a glob tool bound to the given repo root.
func NewGlobTool(repoRoot string) Tool {
	return Tool{
		Definition: llm.ToolDef{
			Name:        "glob",
			Description: "Find files matching a glob pattern. Supports ** for recursive matching (e.g., '**/*.go').",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Glob pattern to match (e.g., '**/*.go', 'src/**/*.ts')",
					},
				},
				"required": []any{"pattern"},
			},
		},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			pattern, _ := args["pattern"].(string)
			if pattern == "" {
				return "Error: missing required parameter 'pattern'", nil
			}

			fsys := os.DirFS(repoRoot)
			matches, err := doublestar.Glob(fsys, pattern, doublestar.WithNoFollow())
			if err != nil {
				return fmt.Sprintf("Error: invalid glob pattern '%s': %s", pattern, err), nil
			}

			// Filter out directories — only return files.
			var files []string
			for _, match := range matches {
				info, statErr := fs.Stat(fsys, match)
				if statErr != nil {
					continue
				}
				if !info.IsDir() {
					files = append(files, match)
				}
			}

			if len(files) == 0 {
				return fmt.Sprintf("No files matched pattern: %s", pattern), nil
			}

			totalCount := len(files)
			if len(files) > MaxGlobResults {
				files = files[:MaxGlobResults]
			}

			var b strings.Builder
			for _, f := range files {
				fmt.Fprintln(&b, f)
			}

			if totalCount > MaxGlobResults {
				fmt.Fprintf(&b, "\n[showing %d of %d matches]", MaxGlobResults, totalCount)
			}

			return b.String(), nil
		},
	}
}
