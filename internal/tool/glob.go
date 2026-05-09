// glob tool: finds files matching glob patterns including ** for recursive matching.

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// MaxGlobResults is the maximum file paths returned from a single glob.
const MaxGlobResults = 200

// NewGlobTool creates a glob tool bound to the given repo root.
func NewGlobTool(repoRoot string) Tool {
	return NewToolFunc(
		"glob",
		"Find files matching a glob pattern. Supports ** for recursive matching (e.g., '**/*.go').",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {
					"type": "string",
					"description": "Glob pattern to match (e.g., '**/*.go', 'src/**/*.ts')"
				}
			},
			"required": ["pattern"]
		}`),
		func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Pattern string `json:"pattern"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}

			if args.Pattern == "" {
				return "Error: missing required parameter 'pattern'", nil
			}

			fsys := os.DirFS(repoRoot)
			matches, err := doublestar.Glob(fsys, args.Pattern, doublestar.WithNoFollow())
			if err != nil {
				return fmt.Sprintf("Error: invalid glob pattern '%s': %s", args.Pattern, err), nil
			}

			// Filter out directories and sensitive files — only return safe files.
			var files []string
			for _, match := range matches {
				info, statErr := fs.Stat(fsys, match)
				if statErr != nil {
					continue
				}
				if info.IsDir() {
					continue
				}
				if IsSensitivePath(match) {
					continue
				}
				files = append(files, match)
			}

			if len(files) == 0 {
				return fmt.Sprintf("No files matched pattern: %s", args.Pattern), nil
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
	)
}
