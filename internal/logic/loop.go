package logic

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/hugoh/upd/internal/logger"
	"github.com/hugoh/upd/internal/status"
	"github.com/hugoh/upd/pkg"
	"github.com/sirupsen/logrus"
)

type (
	Checks []*pkg.Check
	Delays map[bool]time.Duration
)

type Loop struct {
	checks           Checks
	delays           Delays
	downAction       *DownAction
	downActionLoop   *DownActionLoop
	shuffle          bool
	statServer       *status.StatServer
	statServerConfig *status.StatServerConfig
	status           *status.Status
}

func NewLoop(version string) *Loop {
	return &Loop{
		status: status.NewStatus(version),
	}
}

func (l *Loop) Configure(checks Checks,
	delays Delays,
	da *DownAction,
	shuffle bool,
	retention time.Duration,
	statServerConfig *status.StatServerConfig,
) {
	l.checks = checks
	l.delays = delays
	l.downAction = da
	l.shuffle = shuffle
	l.status.SetRetention(retention)
	l.statServerConfig = statServerConfig
}

var ErrDownActionRunning = errors.New("cannot start new DownAction when one is already running")

func (l *Loop) DownActionStart(ctx context.Context) error {
	if l.downActionLoop != nil {
		return ErrDownActionRunning
	}
	l.downActionLoop = l.downAction.Start(ctx)
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

func (l *Loop) ProcessCheck(ctx context.Context, upStatus bool) {
	changed := l.status.Update(upStatus)
	if !changed {
		return
	}
	logger.L.WithField("up", l.status.Up).Info("[Loop] connection status changed")
	if !l.hasDownAction() {
		return
	}
	if upStatus {
		l.DownActionStop()
	} else {
		err := l.DownActionStart(ctx)
		if err != nil {
			logger.L.WithError(err).Error("[Loop] could not start DownAction")
		}
	}
}

func (l *Loop) Run(ctx context.Context) {
	var checker Checker
	l.statServer = status.StartStatServer(l.status, l.statServerConfig)
	for {
		if l.shuffle {
			l.shuffleChecks()
		}
		status, err := pkg.CheckerRun(ctx, checker, l.checks)
		if err == nil {
			l.ProcessCheck(ctx, status)
		} else {
			logger.L.WithError(err).Error("[Loop] error")
		}
		sleepTime := l.delays[l.status.Up]
		logger.L.WithField("wait", sleepTime).Trace("[Loop] waiting for next loop iteration")

		select {
		case <-ctx.Done():
			logger.L.Debug("[Loop] context canceled during sleep, exiting Run()")
			return
		case <-time.After(sleepTime):
		}
	}
}

func (l *Loop) Stop(ctx context.Context) {
	l.DownActionStop()
	if l.statServer != nil {
		l.statServer.StopStatServer(ctx)
	}
}

func (l *Loop) hasDownAction() bool {
	return l.downAction != nil
}

func (l *Loop) shuffleChecks() {
	rand.Shuffle(len(l.checks), func(i, j int) {
		l.checks[i], l.checks[j] = l.checks[j], l.checks[i]
	})
}

type Checker struct{}

func (checker Checker) CheckRun(c pkg.Check) {
	probe := *c.Probe
	logger.L.WithFields(logrus.Fields{
		"probe":    probe,
		"protocol": probe.Scheme(),
		"timeout":  c.Timeout,
	}).Trace("[Check] running")
}

func (checker Checker) ProbeSuccess(report *pkg.Report) {
	logger.L.WithField("report", report).Debug("[Check] success")
}

func (checker Checker) ProbeFailure(report *pkg.Report) {
	logger.L.WithField("report", report).Warn("[Check] failed")
}
