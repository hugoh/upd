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

// BuildReport creates a new report for the given probe.
func BuildReport(p Probe, startTime time.Time) *Report {
	return &Report{
		protocol: p.Scheme(),
		elapsed:  time.Since(startTime),
	}
}

// LogAttrs returns log attributes for logging the report.
func (r *Report) LogAttrs() []any {
	attrs := []any{"protocol", r.protocol, "elapsed", r.elapsed}
	if r.response != "" {
		attrs = append(attrs, "response", r.response)
	} else if r.error != nil {
		attrs = append(attrs, "error", r.error.Error())
	}

	return attrs
}

// Protocol returns the protocol used for the probe (e.g., "http", "tcp", "dns").
func (r *Report) Protocol() string {
	return r.protocol
}

// Response returns the response from the probe.
// For HTTP requests, this is the HTTP status code.
// For TCP probes, this is the local address (IP:port).
// For DNS probes, this is the resolved IP address and DNS resolver.
// Returns empty string if there was an error.
func (r *Report) Response() string {
	return r.response
}

// Elapsed returns the time taken to complete the probe.
func (r *Report) Elapsed() time.Duration {
	return r.elapsed
}

// Error returns any error that occurred during the probe.
// Returns nil if the probe was successful.
func (r *Report) Error() error {
	return r.error
}

// IsError returns true if the probe encountered an error.
func (r *Report) IsError() bool {
	return r.error != nil
}
