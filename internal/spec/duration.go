package spec

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration supports YAML string durations ("600s", "10m").
type Duration struct {
	time.Duration
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag != "!!str" {
		return fmt.Errorf("duration must be a string like \"600s\", got %s", value.Tag)
	}
	var raw string
	if err := value.Decode(&raw); err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	if raw == "" {
		d.Duration = 0
		return nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	d.Duration = parsed
	return nil
}
