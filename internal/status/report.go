package status

import "time"

// JSONIndentSpaces is the indentation used for JSON output.
const JSONIndentSpaces = "  "

// ReportByPeriod contains uptime statistics for a specific time period.
type ReportByPeriod struct {
	Period       ReadableDuration `json:"period"`
	Availability ReadablePercent  `json:"availability"`
	Downtime     ReadableDuration `json:"downTime"`
}

// DownActionStatus contains the current state of the down action loop.
type DownActionStatus struct {
	Iteration     int64            `json:"iteration"`
	SleepTime     ReadableDuration `json:"sleepTime"`
	BackoffCapped bool             `json:"backoffCapped"`
}

// LoopStatus contains the current state of the monitoring loop.
type LoopStatus struct {
	LastSuccess     ReadableDuration `json:"lastSuccess,omitempty"`
	NextCheck       ReadableDuration `json:"nextCheck"`
	Interval        ReadableDuration `json:"interval"`
	TimeSinceUpdate ReadableDuration `json:"timeSinceLastUpdate"`
	TotalChecksRun  int64            `json:"totalChecksRun"`
}

// Report contains the full status report with statistics.
type Report struct {
	Up         bool              `json:"isUp"`
	Stats      []ReportByPeriod  `json:"reports"`
	Loop       *LoopStatus       `json:"loop"`
	DownAction *DownActionStatus `json:"downAction,omitempty"`
	Uptime     ReadableDuration  `json:"updUptime"`
	Version    string            `json:"updVersion"`
	Generated  time.Time         `json:"generatedAt"`
}
