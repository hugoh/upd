package config

import (
	"fmt"
	"time"
)

// Duration is a time.Duration that supports text-based unmarshaling from
// duration strings like "10s", "5m", "1h".
type Duration time.Duration

// UnmarshalText implements the encoding.TextUnmarshaler interface for Duration.
func (d *Duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(text), err)
	}

	*d = Duration(dur)

	return nil
}

// StdDuration returns the Duration as a standard time.Duration.
func (d *Duration) StdDuration() time.Duration {
	return time.Duration(*d)
}
