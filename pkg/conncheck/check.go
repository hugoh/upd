package conncheck

import (
	"time"

	up "github.com/jesusprubio/up/pkg"
	"github.com/sirupsen/logrus"
)

// Connection check definition with protocol, target, timeout
type Check struct {
	Proto   *up.Protocol
	Target  string
	Timeout time.Duration
}

// Run specific connection check and return report
func (c *Check) Probe() up.Report {
	logrus.WithField("check", *c).Trace("[Check] running")
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

/*
Runs a series of checks.
Returns true as soon as one is successful indicating that the connection is up, false otherwise.
*/
func RunChecks(checks []*Check) (bool, error) {
	return RunChecksWithLogger(checks, nil)
}

/*
Runs a series of checks.
Returns true as soon as one is successful indicating that the connection is up, false otherwise.
Logs output using logrus.Logger instance
*/
func RunChecksWithLogger(checks []*Check, logger *logrus.Logger) (bool, error) {
	for _, check := range checks {
		report := check.Probe()
		if report.Error != nil {
			if logger != nil {
				logger.WithField("report", report).Warn("[Check] check failed")
			}
			continue
		}
		if logger != nil {
			logger.WithField("report", report).Debug("[Check] check run")
		}
		return true, nil
	}
	return false, nil
}
