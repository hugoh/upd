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
	downActionLoop *DownActionLoop
	initialized    bool
	isUp           bool
}

func (l *Loop) hasDownAction() bool {
	return l.DownAction != nil
}

func (l *Loop) DownActionStart() error {
	if l.downActionLoop != nil {
		return errors.New("cannot start new DownAction when one is already running")
	}
	logrus.WithField("da", l.DownAction).Debug("[Loop] starting DownAction")
	l.downActionLoop = l.DownAction.Start()
	return nil
}

func (l *Loop) DownActionStop() {
	if l.downActionLoop == nil {
		// Nothing to stop
		return
	}
	logrus.Debug("[Loop] stopping DownAction")
	l.downActionLoop.Stop()
	l.downActionLoop = nil
}

// Returns true if it changed
func (l *Loop) reportUpness(result bool) bool {
	if l.initialized && result == l.isUp {
		return false
	}
	if !l.initialized {
		l.initialized = true
	}
	l.isUp = result
	return true
}

func (l *Loop) ProcessCheck(upStatus bool) {
	changed := l.reportUpness(upStatus)
	logrus.WithField("up", l.isUp).Info("[Loop] connection status changed")
	if changed && l.hasDownAction() {
		if upStatus {
			l.DownActionStop()
		} else {
			err := l.DownActionStart()
			if err != nil {
				logrus.WithField("err", err).Error("[Loop] could not start DownAction")
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
			logrus.WithField("err", err).Error("[Loop] error")
		}
		sleepTime := l.Delays[l.isUp]
		logrus.WithField("wait", sleepTime).Debugf("[Loop] waiting for next loop iteration")
		time.Sleep(sleepTime)
	}
}
