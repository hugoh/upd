package check

import (
	"context"
	"time"
)

// Probe represents a network connectivity test for a specific protocol.
//
// Probe implementations must be thread-safe and can be used concurrently.
type Probe interface {
	// Execute runs the connectivity test with the given context and timeout.
	// Returns a Report containing the result of the probe.
	//
	// The context parameter is used for overall cancellation control.
	// The timeout parameter specifies the maximum duration for the probe operation.
	// Implementations should respect both the context and timeout values.
	//
	// Example:
	//
	// ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	// defer cancel()
	// report := probe.Execute(ctx, 2*time.Second)
	// if report.error != nil {
	// // handle error
	// }
	Execute(ctx context.Context, timeout time.Duration) *Report

	// Scheme returns the protocol scheme (e.g., "http", "https", "tcp", "dns").
	// This is used for logging and identifying the type of probe.
	//
	// Example: "http", "https", "tcp", "dns"
	Scheme() string
}
