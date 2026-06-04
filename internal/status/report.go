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

// Report contains the full status report with statistics.
type Report struct {
	Up         bool             `json:"isUp"`
	Stats      []ReportByPeriod `json:"reports"`
	CheckCount int64            `json:"totalChecksRun"`
	LastUpdate ReadableDuration `json:"timeSinceLastUpdate"`
	Uptime     ReadableDuration `json:"updUptime"`
	Version    string           `json:"updVersion"`
	Generated  time.Time        `json:"generatedAt"`
}
