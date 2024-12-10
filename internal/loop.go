package internal

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/hugoh/upd/pkg"
	"github.com/sirupsen/logrus"
)

type (
	Checks []*pkg.Check
	Delays map[bool]time.Duration
)

type Loop struct {
	Checks         Checks
	Delays         Delays
	DownAction     *DownAction
	Shuffle        bool
	downActionLoop *DownActionLoop
	Status         *Status
}

func NewLoop(checks Checks, delays Delays, da *DownAction, shuffle bool, status *Status) *Loop {
	return &Loop{
		Checks:     checks,
		Delays:     delays,
		DownAction: da,
		Shuffle:    shuffle,
		Status:     status,
	}
}

func (l *Loop) hasDownAction() bool {
	return l.DownAction != nil
}

var ErrDownActionRunning = errors.New("cannot start new DownAction when one is already running")

func (l *Loop) DownActionStart() error {
	if l.downActionLoop != nil {
		return ErrDownActionRunning
	}
	l.downActionLoop = l.DownAction.Start()
	return nil
}

func (l *Loop) DownActionStop() {
	if l.downActionLoop == nil {
		// Nothing to stop
		return
	}
	l.downActionLoop.Stop()
	l.downActionLoop = nil
}

func (l *Loop) ProcessCheck(upStatus bool) {
	changed := l.Status.Update(upStatus)
	if !changed {
		return
	}
	logger.WithField("up", l.Status.Up).Info("[Loop] connection status changed")
	if !l.hasDownAction() {
		return
	}
	if upStatus {
		l.DownActionStop()
	} else {
		err := l.DownActionStart()
		if err != nil {
			logger.WithError(err).Error("[Loop] could not start DownAction")
		}
	}
}

func (l *Loop) shuffleChecks() {
	rand.Shuffle(len(l.Checks), func(i, j int) {
		l.Checks[i], l.Checks[j] = l.Checks[j], l.Checks[i]
	})
}

type Checker struct{}

func (checker Checker) CheckRun(c pkg.Check) {
	probe := *c.Probe
	logger.WithFields(logrus.Fields{
		"probe":    probe,
		"protocol": probe.Scheme(),
		"timeout":  c.Timeout,
	}).Trace("[Check] running")
}

func (checker Checker) ProbeSuccess(report *pkg.Report) {
	logger.WithField("report", report).Debug("[Check] success")
}

func (checker Checker) ProbeFailure(report *pkg.Report) {
	logger.WithField("report", report).Warn("[Check] failed")
}

func (l *Loop) Run() {
	var checker Checker
	for {
		if l.Shuffle {
			l.shuffleChecks()
		}
		status, err := pkg.CheckerRun(checker, l.Checks)
		if err == nil {
			l.ProcessCheck(status)
		} else {
			logger.WithError(err).Error("[Loop] error")
		}
		sleepTime := l.Delays[l.Status.Up]
		logger.WithField("wait", sleepTime).Trace("[Loop] waiting for next loop iteration")
		time.Sleep(sleepTime)
	}
}
