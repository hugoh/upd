// Package types provides shared types for configuration handling.
package types

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that supports YAML unmarshaling from duration
// strings like "10s", "5m", "1h".
type Duration time.Duration

// UnmarshalYAML implements the yaml.Unmarshaler interface for Duration.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var str string

	if err := value.Decode(&str); err != nil {
		return fmt.Errorf("failed to decode duration value: %w", err)
	}

	dur, err := time.ParseDuration(str)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", str, err)
	}

	*d = Duration(dur)

	return nil
}

// StdDuration returns the Duration as a standard time.Duration.
func (d *Duration) StdDuration() time.Duration {
	return time.Duration(*d)
}
