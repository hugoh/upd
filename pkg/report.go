// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package pkg

import (
	"time"
)

// Report is the result of a connection attempt.
//
// Only one of the properties 'Response' or 'Error' is set.
type Report struct {
	// Protocol used to connect to.
	Protocol string
	// Target used to connect to.
	Response string
	// Response time.
	Elapsed time.Duration
	// Network error.
	Error error
}

func BuildReport(p Probe, startTime time.Time) *Report {
	return &Report{ //nolint:exhaustruct
		Protocol: p.Scheme(),
		Elapsed:  time.Since(startTime),
	}
}
