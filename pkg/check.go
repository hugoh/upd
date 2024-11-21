package pkg

import (
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
func (c *Check) RunProbe(checker Checker) *Report {
	checker.CheckRun(*c)
	p := *c.Probe
	return p.Probe(c.Timeout)
}

type NullChecker struct{}

func (c NullChecker) CheckRun(_ Check)       {}
func (c NullChecker) ProbeSuccess(_ *Report) {}
func (c NullChecker) ProbeFailure(_ *Report) {}

/*
Runs a series of checks.
Returns true as soon as one is successful indicating that the connection is up, false otherwise.
*/
func RunChecks(checks []*Check) (bool, error) {
	var nc NullChecker
	return CheckerRun(nc, checks)
}

/*
Runs a series of checks utilizing a Checker interface for handling probe return.
Returns true as soon as one is successful indicating that the connection is up, false otherwise.
Logs output using logrus.Logger instance
*/
func CheckerRun(checker Checker, checks []*Check) (bool, error) {
	for _, check := range checks {
		report := check.RunProbe(checker)
		if report.Error != nil {
			checker.ProbeFailure(report)
			continue
		}
		checker.ProbeSuccess(report)
		return true, nil
	}
	return false, nil
}
