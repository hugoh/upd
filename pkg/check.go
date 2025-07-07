package pkg

import (
	"context"
	"time"
)

// Connection check definition with protocol, target, timeout
type Check struct {
	Probe   *Probe
	Timeout time.Duration
}

// Interface to act on probe success or failure when running checks
type Checker interface {
	CheckRun(c Check)
	ProbeSuccess(report *Report)
	ProbeFailure(report *Report)
}

// Run specific connection check and return report
func (c *Check) RunProbe(ctx context.Context, checker Checker) *Report {
	checker.CheckRun(*c)
	p := *c.Probe
	return p.Probe(ctx, c.Timeout)
}

type NullChecker struct{}

func (c NullChecker) CheckRun(_ Check)       {}
func (c NullChecker) ProbeSuccess(_ *Report) {}
func (c NullChecker) ProbeFailure(_ *Report) {}

/*
Runs a series of checks.
Returns true as soon as one is successful indicating that the connection is up, false otherwise.
*/
func RunChecks(ctx context.Context, checkListIterator CheckListIterator) (bool, error) {
	var nc NullChecker
	return CheckerRun(ctx, nc, checkListIterator)
}

/*
Runs a series of checks utilizing a Checker interface for handling probe return.
Returns true as soon as one is successful indicating that the connection is up, false otherwise.
Logs output using logrus.Logger instance
*/
func CheckerRun(ctx context.Context, checker Checker, checkListIterator CheckListIterator) (bool, error) {
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
