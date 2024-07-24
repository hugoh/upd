package internal

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
)

type DownAction struct {
	After        time.Duration
	Every        time.Duration
	BackoffLimit time.Duration
	Exec         string
	ExecArgs     []string
	running      bool
	cancelFunc   context.CancelFunc
}

type DaIteration struct {
	Iteration    int
	SleepTime    time.Duration
	LimitReached bool
}

const BackoffFactor = 1.5

// Only return an error if the command cannot be run.
func (da *DownAction) Execute() error {
	cmd := exec.Command(da.Exec, da.ExecArgs...) // #nosec G204
	logrus.WithField("exec", da.Exec).Debugf("[DownAction] Executing")
	err := cmd.Start()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"exec": da.Exec,
			"err":  err,
		}).Error("[DownAction] Failed to run")
		return fmt.Errorf("failed to execute DownAction: %w", err)
	}
	go func() {
		err := cmd.Wait()
		logrus.WithFields(logrus.Fields{
			"command": da.Exec,
			"err":     err,
		}).Warn("[DownAction] Error")
	}()
	return nil
}

func (da *DownAction) isRunning() bool {
	return da.running
}

func (it *DaIteration) iterate(da *DownAction) {
	switch it.Iteration {
	case 0:
		it.SleepTime = da.After
	case 1:
		it.SleepTime = da.Every
	default:
		if !it.LimitReached {
			it.SleepTime = time.Duration(BackoffFactor * float64(it.SleepTime))
			if da.BackoffLimit != 0 && it.SleepTime >= da.BackoffLimit {
				it.SleepTime = da.BackoffLimit
				it.LimitReached = true
			}
		}
	}
	it.Iteration++
	logrus.WithFields(logrus.Fields{
		"iteration":    it.Iteration,
		"sleepTime":    it.SleepTime,
		"limitReached": it.LimitReached,
	}).Debug("[DownAction] Iteration details")
}

func (da *DownAction) run(ctx context.Context) {
	da.running = true
	it := &DaIteration{} //nolint:exhaustruct
	it.iterate(da)
	for {
		select {
		case <-ctx.Done():
			logrus.Debug("[DownAction] canceled")
			da.running = false
			return
		case <-time.After(it.SleepTime):
		}
		_ = da.Execute() //nolint:errcheck
		if da.Every > 0 {
			it.iterate(da)
		} else {
			break
		}
	}
}

func (da *DownAction) Start() {
	logrus.SetLevel(logrus.DebugLevel)
	var ctx context.Context
	ctx, da.cancelFunc = context.WithCancel(context.Background())
	logrus.Debug("[DownAction] kicking off run loop")
	go da.run(ctx)
}

func (da *DownAction) Stop() {
	if da.cancelFunc != nil {
		logrus.Debug("[DownAction] sending shutdown signal")
		da.cancelFunc()
	}
}
