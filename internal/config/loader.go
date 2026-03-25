package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kilupskalvis/jerry/internal/errors"
	"github.com/kilupskalvis/jerry/internal/pipeline"
)

// FileConfig represents the parsed .jerry/config.yaml file.
type FileConfig struct {
	Defaults DefaultsConfig `yaml:"defaults,omitempty"`
}

// DefaultsConfig holds default values for agent and pipeline configuration.
type DefaultsConfig struct {
	Model         string            `yaml:"model,omitempty"`
	Timeout       pipeline.Duration `yaml:"timeout,omitempty"`
	MaxIterations int               `yaml:"max_iterations,omitempty"`
}

// LoadFileConfig reads and parses a config YAML file from the given directory.
// Returns an empty FileConfig (not an error) if the file doesn't exist.
func LoadFileConfig(dir, filename string) (*FileConfig, error) {
	configPath := filepath.Join(dir, filename)

	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return &FileConfig{}, nil
		}
		return nil, errors.Wrap(errors.CodeConfigInvalid,
			fmt.Sprintf("failed to read config file %q", configPath), readErr)
	}

	var cfg FileConfig
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		return nil, errors.Wrap(errors.CodeConfigInvalid,
			fmt.Sprintf("invalid config file %q", configPath), unmarshalErr)
	}

	return &cfg, nil
}

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

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip optional "export " prefix.
		line = strings.TrimPrefix(line, "export ")

		// Split on first =.
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
