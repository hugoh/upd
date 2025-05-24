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
	// protocol used to connect to.
	protocol string
	// Target used to connect to.
	response string
	// Response time.
	elapsed time.Duration
	// Network error.
	error error
}

func BuildReport(p Probe, startTime time.Time) *Report {
	return &Report{
		protocol: p.Scheme(),
		elapsed:  time.Since(startTime),
	}
}
