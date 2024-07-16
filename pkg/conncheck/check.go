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
	logrus.WithField("check", *c).Debug("[Check] Running")
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
	for _, check := range checks {
		report := check.Probe()
		if report.Error != nil {
			logrus.WithField("report", report).Warn("[Check] Check failed")
			continue
		}
		logrus.WithField("report", report).Debug("[Check] Check run")
		return true, nil
	}
	return false, nil
}
