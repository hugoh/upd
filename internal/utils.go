package internal

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

func (p ReadablePercent) MarshalJSON() ([]byte, error) {
	const Hundred = 100
	if p == -1.0 {
		return json.Marshal("Not computed") //nolint:wrapcheck
	}
	return json.Marshal(fmt.Sprintf("%.2f %%", p*Hundred)) //nolint:wrapcheck
}

func formatDuration(d time.Duration) string {
	str := d.Truncate(time.Second).String()
	if str == "0s" {
		return str
	}
	// Remove trailing "0s" or "0m0s"
	if strings.HasSuffix(str, "m0s") {
		str = strings.TrimSuffix(str, "0s")
	}
	if strings.HasSuffix(str, "h0m") {
		str = strings.TrimSuffix(str, "0m")
	}
	return str
}

func (d ReadableDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(formatDuration(time.Duration(d))) //nolint:wrapcheck
}
