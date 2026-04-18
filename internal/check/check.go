// Package check provides network connectivity checking functionality.
package check

import (
	"context"
	"time"
)

// Check represents a connection check definition with a probe and timeout.
//
// Example:
//
//	check := &check.Check{
//	    Probe: check.NewHTTPProbe("https://example.com"),
//	    Timeout: 10 * time.Second,
//	}
type Check struct {
	Probe   Probe         // The probe to execute for this check
	Timeout time.Duration // Maximum duration to wait for the probe to complete
}

// Checker handles lifecycle events for a check execution.
//
// Implement this interface to customize behavior during check runs.
// Common use cases include:
// - Custom logging
// - Metrics collection
// - Alerting
// - Status tracking
//
// Example - Custom Logging Checker:
//
// type LoggingChecker struct {}
//
//	func (l *LoggingChecker) CheckRun(c Check) {
//		fmt.Printf("Running check: %s (timeout: %v)\n", c.Probe.Scheme(), c.Timeout)
//	}
//
//	func (l *LoggingChecker) ProbeSuccess(report *Report) {
//	    fmt.Printf("Success: %s, elapsed: %v\n", report.Response(), report.Elapsed())
//	}
//
//	func (l *LoggingChecker) ProbeFailure(report *Report) {
//	    fmt.Printf("Failed: %s, error: %v\n", report.Protocol(), report.Error())
//	}
type Checker interface {
	// CheckRun is called before executing a probe.
	// Use this to log check start or prepare state.
	CheckRun(c Check)

	// ProbeSuccess is called when a probe succeeds.
	// Use this to update metrics, log results, or trigger alerts.
	ProbeSuccess(report *Report)

	// ProbeFailure is called when a probe fails.
	// Use this to log errors, update failure counters, or trigger alerts.
	ProbeFailure(report *Report)
}

// RunProbe executes the check and returns a report.
func (c *Check) RunProbe(ctx context.Context, checker Checker) *Report {
	checker.CheckRun(*c)

	return c.Probe.Execute(ctx, c.Timeout)
}

// NullChecker is a no-op implementation of the Checker interface.
//
// Use this when you don't need any special handling of check lifecycle events.
// All methods are empty operations that do nothing.
//
// Example:
//
//	checker := &check.NullChecker{}
//	report := check.RunProbe(ctx, checker)
type NullChecker struct{}

// CheckRun is a no-op implementation of Checker.CheckRun.
func (NullChecker) CheckRun(_ Check) {}

// ProbeSuccess is a no-op implementation of Checker.ProbeSuccess.
func (NullChecker) ProbeSuccess(_ *Report) {}

// ProbeFailure is a no-op implementation of Checker.ProbeFailure.
func (NullChecker) ProbeFailure(_ *Report) {}

// RunChecks executes a series of checks using a NullChecker.
//
// Returns true as soon as one check is successful, indicating that the
// connection is up. Returns false if all checks fail.
//
// This is a convenience wrapper around CheckerRun that uses NullChecker.
//
// Example:
//
//	success, err := check.RunChecks(ctx, checkList.GetIterator())
//	if success {
//	    fmt.Println("Connection is up")
//	} else {
//	    fmt.Println("Connection is down")
//	}
func RunChecks(ctx context.Context, checkListIterator ListIterator) (bool, error) {
	var nc NullChecker

	return CheckerRun(ctx, nc, checkListIterator)
}

// CheckerRun executes a series of checks using the provided Checker interface.
//
// Returns true as soon as one check is successful, indicating that the
// connection is up. Returns false if all checks fail.
//
// The Checker interface methods are called at appropriate times:
// - CheckRun before each probe is executed
// - ProbeSuccess after a successful probe
// - ProbeFailure after a failed probe
//
// This allows custom logging, metrics collection, or alerting during
// check execution.
//
// Example:
//
//	checker := &LoggingChecker{}
//	success, err := check.CheckerRun(ctx, checker, checkList.GetIterator())
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - checker: Implementation of Checker interface for lifecycle callbacks
//   - checkListIterator: Iterator over checks to execute
//
// Returns:
//   - bool: true if any check succeeded, false if all failed
//   - error: any error that occurred during iteration (not individual probe failures)
func CheckerRun(
	ctx context.Context,
	checker Checker,
	checkListIterator ListIterator,
) (bool, error) {
	for {
		check := checkListIterator.Fetch()
		if check == nil {
			return false, nil // All checks failed
		}

		report := check.RunProbe(ctx, checker)
		if report.error != nil {
			checker.ProbeFailure(report)

			continue
		}

		checker.ProbeSuccess(report)

		return true, nil
	}
}
