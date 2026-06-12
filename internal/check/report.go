// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package check

import (
	"log/slog"
	"time"
)

// Report is the result of a connection attempt.
//
// Only one of the properties 'Response' or 'Error' is set.
type Report struct {
	protocol string
	target   string
	response string
	elapsed  time.Duration
	error    error
}

// BuildReport creates a new report for the given probe.
func BuildReport(p Probe, startTime time.Time) *Report {
	return &Report{
		protocol: p.Scheme(),
		target:   p.Target(),
		elapsed:  time.Since(startTime),
	}
}

// LogAttrs returns structured log attributes for the report.
func (r *Report) LogAttrs() slog.Attr {
	attrs := []any{
		slog.String("protocol", r.protocol),
		slog.String("target", r.target),
		slog.Duration("elapsed", r.elapsed),
	}
	if r.response != "" {
		attrs = append(attrs, slog.String("response", r.response))
	} else if r.error != nil {
		attrs = append(attrs, slog.Any("error", r.error))
	}

	return slog.Group("report", attrs...)
}
