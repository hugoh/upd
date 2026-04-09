package status

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type (
	// ReadablePercent is a float64 formatted as a percentage for JSON output.
	ReadablePercent float64
	// ReadableDuration is a time.Duration formatted for human-readable JSON output.
	ReadableDuration time.Duration
)

const (
	// PercentMultiplier converts decimal to percentage.
	PercentMultiplier = 100
	// NotComputedMsg is displayed when a value cannot be computed.
	NotComputedMsg = "Not computed"
	// PercentFormat is the format string for percentage display.
	PercentFormat = "%.2f %%"
	// ZeroString is the string representation of zero duration.
	ZeroString = "0s"
	// TrailingZeroSSuffix is the trailing zero seconds suffix.
	TrailingZeroSSuffix = "0s"
	// TrailingZeroMSuffix is the trailing zero minutes suffix.
	TrailingZeroMSuffix = "m0s"
	// TrailingZeroHSuffix is the trailing zero hours suffix.
	TrailingZeroHSuffix = "h0m"
	// TrailingZeroHMSuffix is the trailing zero hours/minutes suffix.
	TrailingZeroHMSuffix = "0m"
)

// MarshalJSON formats the percentage for JSON output.
func (p ReadablePercent) MarshalJSON() ([]byte, error) {
	if p == -1.0 {
		return json.Marshal(NotComputedMsg) //nolint:wrapcheck
	}

	return json.Marshal(fmt.Sprintf(PercentFormat, p*PercentMultiplier)) //nolint:wrapcheck
}

func formatDuration(d time.Duration) string {
	str := d.Truncate(time.Second).String()
	if str == ZeroString {
		return str
	}
	// Remove trailing "0s" or "0m0s"
	if strings.HasSuffix(str, TrailingZeroMSuffix) {
		str = strings.TrimSuffix(str, TrailingZeroSSuffix)
	}

	if strings.HasSuffix(str, TrailingZeroHSuffix) {
		str = strings.TrimSuffix(str, TrailingZeroHMSuffix)
	}

	return str
}

// MarshalJSON formats the duration for JSON output.
func (d ReadableDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(formatDuration(time.Duration(d))) //nolint:wrapcheck
}
