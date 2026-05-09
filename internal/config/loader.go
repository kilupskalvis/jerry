package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv reads a dotenv file and returns the key-value pairs.
// Returns an empty map (not an error) if the file doesn't exist.
func LoadDotEnv(dir, filename string) (map[string]string, error) {
	envPath := filepath.Join(dir, filename)

	file, openErr := os.Open(envPath)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to open %s: %w", envPath, openErr)
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

		key = strings.TrimSpace(key)
		value = stripQuotes(value)

		env[key] = value
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("error reading %s: %w", envPath, scanErr)
	}

	return env, nil
}

// stripQuotes removes matching surrounding double or single quotes.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
