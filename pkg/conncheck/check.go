package conncheck

import (
	"time"

	up "github.com/jesusprubio/up/pkg"
)

// Connection check definition with protocol, target, timeout
type Check struct {
	Proto   *up.Protocol
	Target  string
	Timeout time.Duration
}

// Interface to act on probe success or failure when running checks
type Checker interface {
	CheckRun(c Check)
	ProbeSuccess(report up.Report)
	ProbeFailure(report up.Report)
}

// Run specific connection check and return report
func (c *Check) Probe(checker Checker) up.Report {
	checker.CheckRun(*c)
	start := time.Now()
	extra, err := c.Proto.Probe(c.Target, c.Timeout)
	report := up.Report{
		ProtocolID: c.Proto.ID,
		RHost:      c.Target,
		Time:       time.Since(start),
		Error:      err,
		Extra:      extra,
	}
	return report
}

type NullChecker struct{}

func (c NullChecker) CheckRun(_ up.Report)     {}
func (c NullChecker) ProbeSuccess(_ up.Report) {}
func (c NullChecker) ProbeFailure(_ up.Report) {}

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
		report := check.Probe()
		if report.Error != nil {
			checker.ProbeFailure(report)
			continue
		}
		checker.ProbeSuccess(report)
		return true, nil
	}
	return false, nil
}
