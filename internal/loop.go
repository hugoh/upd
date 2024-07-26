package internal

import (
	"errors"
	"time"

	"github.com/hugoh/upd/pkg/conncheck"
	"github.com/sirupsen/logrus"
)

type Loop struct {
	Checks         []*conncheck.Check
	Delays         map[bool]time.Duration
	DownAction     *DownAction
	DownActionLoop *DownActionLoop
	initialized    bool
	isUp           bool
}

// Returns true if it changed
func (l *Loop) reportUpness(result bool) bool {
	if !l.initialized || result != l.isUp {
		l.isUp = result
		return true
	}
	return false
}

func (l *Loop) hasDownAction() bool {
	return l.DownAction != nil
}

func (l *Loop) DownActionStart() error {
	if l.DownActionLoop != nil {
		return errors.New("cannot start new DownAction when one is already running")
	}
	logrus.WithField("da", l.DownAction).Debug("[Loop] Starting DownAction")
	l.DownActionLoop = l.DownAction.Start()
	return nil
}

func (l *Loop) DownActionStop() {
	if l.DownActionLoop == nil {
		// Nothing to stop
		return
	}
	logrus.Debug("[Loop] Stopping DownAction")
	l.DownActionLoop.Stop()
	l.DownActionLoop = nil
}

func (l *Loop) ProcessCheck(upStatus bool) {
	changed := l.reportUpness(upStatus)
	logrus.WithField("up", l.isUp).Info("[Loop] Connection status changed")
	if changed && l.hasDownAction() {
		if upStatus {
			l.DownActionStop()
		} else {
			err := l.DownActionStart()
			if err != nil {
				logrus.WithField("err", err).Error("[Loop] Could not start DownAction")
			}
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
		logrus.WithField("wait", sleepTime).Debugf("[Loop] Waiting for next loop iteration")
		time.Sleep(sleepTime)
	}
}
