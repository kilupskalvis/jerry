package spec

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	jerrerr "github.com/kilupskalvis/jerry/internal/errors"
)

// FindJerryDir walks up from startDir looking for a .jerry/ directory,
// returning the directory and the repository root that contains it.
func FindJerryDir(startDir string) (jerryDir, repoRoot string, err error) {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", jerrerr.Wrap(jerrerr.CodeJerryDirNotFound,
			"failed to resolve absolute path", err)
	}

	for {
		candidate := filepath.Join(current, ".jerry")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", "", jerrerr.New(jerrerr.CodeJerryDirNotFound,
				"no .jerry/ directory found (searched from "+startDir+" to filesystem root)")
		}
		current = parent
	}
}

// LoadDotEnv reads a dotenv file into a key-value map. A missing file
// returns an empty map, not an error.
func LoadDotEnv(dir, filename string) (map[string]string, error) {
	envPath := filepath.Join(dir, filename)
	file, openErr := os.Open(envPath)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("opening %s: %w", envPath, openErr)
	}
	defer func() { _ = file.Close() }()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		env[strings.TrimSpace(key)] = stripQuotes(value)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("reading %s: %w", envPath, scanErr)
	}
	return env, nil
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
