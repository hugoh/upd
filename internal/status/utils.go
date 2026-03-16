package status

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type (
	ReadablePercent  float64
	ReadableDuration time.Duration
)

const (
	PercentMultiplier    = 100
	NotComputedMsg       = "Not computed"
	PercentFormat        = "%.2f %%"
	ZeroString           = "0s"
	TrailingZeroSSuffix  = "0s"
	TrailingZeroMSuffix  = "m0s"
	TrailingZeroHSuffix  = "h0m"
	TrailingZeroHMSuffix = "0m"
)

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

func (d ReadableDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(formatDuration(time.Duration(d))) //nolint:wrapcheck
}
