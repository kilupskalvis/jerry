// list_directory tool: lists directory entries with [dir]/[file] prefixes.

package tools

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/kilupskalvis/jerry/internal/llm"
)

// MaxListDirEntries is the maximum entries returned from list_directory.
const MaxListDirEntries = 200

// NewListDirectoryTool creates a list_directory tool bound to the given repo root.
func NewListDirectoryTool(repoRoot string) Tool {
	return Tool{
		Definition: llm.ToolDef{
			Name:        "list_directory",
			Description: "List the contents of a directory. Shows entries as [dir] or [file] with directories listed first.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the directory (relative to repository root)",
					},
				},
				"required": []any{"path"},
			},
		},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}

			absPath, pathErr := resolvePath(repoRoot, path)
			if pathErr != "" {
				return pathErr, nil
			}

			info, err := os.Stat(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Sprintf("Error: directory not found: %s", path), nil
				}
				return fmt.Sprintf("Error: cannot access '%s': %s", path, err), nil
			}

			if !info.IsDir() {
				return fmt.Sprintf("Error: '%s' is not a directory", path), nil
			}

			entries, err := os.ReadDir(absPath)
			if err != nil {
				return fmt.Sprintf("Error: cannot read directory '%s': %s", path, err), nil
			}

			// Separate directories and files, sort each alphabetically.
			var dirs, files []string
			for _, entry := range entries {
				if entry.IsDir() {
					dirs = append(dirs, entry.Name())
				} else {
					files = append(files, entry.Name())
				}
			}
			sort.Strings(dirs)
			sort.Strings(files)

			totalCount := len(dirs) + len(files)
			var b strings.Builder

			written := 0
			for _, name := range dirs {
				if written >= MaxListDirEntries {
					break
				}
				fmt.Fprintf(&b, "[dir]  %s\n", name)
				written++
			}
			for _, name := range files {
				if written >= MaxListDirEntries {
					break
				}
				fmt.Fprintf(&b, "[file] %s\n", name)
				written++
			}

			if totalCount > MaxListDirEntries {
				fmt.Fprintf(&b, "\n[showing %d of %d entries]", MaxListDirEntries, totalCount)
			}

			return b.String(), nil
		},
	}
}
