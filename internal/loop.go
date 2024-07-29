package internal

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/hugoh/upd/pkg/conncheck"
	"github.com/sirupsen/logrus"
)

type Loop struct {
	Checks         []*conncheck.Check
	Delays         map[bool]time.Duration
	DownAction     *DownAction
	Shuffle        bool
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
	if !changed {
		return
	}
	logrus.WithField("up", l.isUp).Info("[Loop] connection status changed")
	if !l.hasDownAction() {
		return
	}
	if upStatus {
		l.DownActionStop()
	} else {
		err := l.DownActionStart()
		if err != nil {
			logrus.WithField("err", err).Error("[Loop] could not start DownAction")
		}
	}
}

func (l *Loop) shuffleChecks() {
	rand.Shuffle(len(l.Checks), func(i, j int) {
		l.Checks[i], l.Checks[j] = l.Checks[j], l.Checks[i]
	})
}

func (l *Loop) Run() {
	for {
		if l.Shuffle {
			l.shuffleChecks()
		}
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
