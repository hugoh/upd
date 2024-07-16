package internal

import (
	"time"

	"github.com/hugoh/upd/pkg/conncheck"
	"github.com/sirupsen/logrus"
)

type Loop struct {
	Checks      []*conncheck.Check
	Delays      map[bool]time.Duration
	DownAction  *DownAction
	initialized bool
	isUp        bool
}

// Returns true if it changed
func (l *Loop) reportUpness(result bool) bool {
	if !l.initialized || result != l.isUp {
		l.isUp = result
		return true
	}
	return false
}

func (l *Loop) ProcessCheck(status bool) {
	changed := l.reportUpness(status)
	logrus.WithField("up", l.isUp).Info("[Loop] Connection status changed")
	if changed {
		if status {
			logrus.Debug("[Loop] Stopping DownAction")
			l.DownAction.Stop()
		} else {
			logrus.WithField("da", l.DownAction).Debug("[Loop] Starting DownAction")
			l.DownAction.Start()
		}
	}
}

func (l *Loop) Run() {
	for {
		status, err := conncheck.RunChecks(l.Checks)
		if err == nil {
			l.ProcessCheck(status)
		} else {
			logrus.WithField("err", err).Error("[Loop] Error")
		}
		sleepTime := l.Delays[l.isUp]
		logrus.WithField("wait", sleepTime.Seconds()).Debugf("[Loop] Waiting for next loop iteration")
		time.Sleep(sleepTime)
	}
}
