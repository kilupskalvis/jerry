// search_codebase tool: regex search across file contents.

package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// MaxSearchResults is the maximum matching lines returned.
	MaxSearchResults = 100

	// MaxSearchFileSize skips files larger than this when searching.
	MaxSearchFileSize = 1 * 1024 * 1024 // 1MB
)

// skipDirs lists directories excluded from search.
var skipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
}

// skipPaths lists path prefixes excluded from search (checked with HasPrefix).
var skipPaths = []string{
	".jerry" + string(filepath.Separator) + "runs",
	".jerry" + string(filepath.Separator) + "cache",
}

// NewSearchTool creates a search_codebase tool bound to the given repo root.
func NewSearchTool(repoRoot string) Tool {
	return NewToolFunc(
		"search_codebase",
		"Search file contents using a regular expression pattern. Returns matching lines as file:line:content.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "Regular expression pattern to search for"
				},
				"glob": {
					"type": "string",
					"description": "Optional glob pattern to filter files (e.g., '*.go')"
				}
			},
			"required": ["query"]
		}`),
		func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Glob  string `json:"glob"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return fmt.Sprintf("Error: invalid input: %v", err), nil
			}

			if args.Query == "" {
				return "Error: missing required parameter 'query'", nil
			}

			re, err := regexp.Compile(args.Query)
			if err != nil {
				return fmt.Sprintf("Error: invalid regex '%s': %s", args.Query, err), nil
			}

			var matches []string
			totalMatches := 0

			walkErr := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, walkDirErr error) error {
				if walkDirErr != nil {
					return nil // Skip entries with errors.
				}

				relPath, _ := filepath.Rel(repoRoot, path)

				// Skip excluded directories.
				if d.IsDir() {
					if _, skip := skipDirs[d.Name()]; skip {
						return filepath.SkipDir
					}
					for _, prefix := range skipPaths {
						if strings.HasPrefix(relPath, prefix) {
							return filepath.SkipDir
						}
					}
					return nil
				}

				// Apply glob filter if specified.
				if args.Glob != "" {
					matched, matchErr := filepath.Match(args.Glob, d.Name())
					if matchErr != nil || !matched {
						return nil
					}
				}

				// Skip sensitive files.
				if IsSensitivePath(relPath) {
					return nil
				}

				// Skip large files.
				info, statErr := d.Info()
				if statErr != nil || info.Size() > MaxSearchFileSize {
					return nil
				}

				// Skip binary files (check first 512 bytes for null bytes).
				if isBinary(path) {
					return nil
				}

				// Search the file.
				file, openErr := os.Open(path)
				if openErr != nil {
					return nil
				}
				defer func() { _ = file.Close() }()

				scanner := bufio.NewScanner(file)
				lineNum := 0
				for scanner.Scan() {
					lineNum++
					line := scanner.Text()
					if re.MatchString(line) {
						totalMatches++
						if len(matches) < MaxSearchResults {
							matches = append(matches, fmt.Sprintf("%s:%d: %s", relPath, lineNum, line))
						}
					}
				}

				return nil
			})

			if walkErr != nil {
				return fmt.Sprintf("Error: search failed: %s", walkErr), nil
			}

			if len(matches) == 0 {
				return fmt.Sprintf("No matches found for pattern: %s", args.Query), nil
			}

			var b strings.Builder
			for _, m := range matches {
				fmt.Fprintln(&b, m)
			}

			if totalMatches > MaxSearchResults {
				fmt.Fprintf(&b, "\n[showing %d of %d matches]", MaxSearchResults, totalMatches)
			}

			return b.String(), nil
		},
	)
}

// isBinary checks if a file is likely binary by looking for null bytes
// in the first 512 bytes.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}
