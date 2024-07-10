package internal

import (
	"context"
	"time"

	"github.com/hugoh/upd/pkg/conncheck"
	"github.com/sirupsen/logrus"
)

type Loop struct {
	Checks     []*conncheck.Check
	Delays     map[bool]time.Duration
	DownAction *DownAction
}

var initialized bool
var isUp bool
var ctx context.Context

// Returns true if it changed
func reportUpness(result bool) bool {
	if !initialized || result != isUp {
		isUp = result
		return true
	}
	return false
}

func (l *Loop) Run() {
	for {
		status, err := conncheck.RunChecks(l.Checks)
		if err == nil {
			changed := reportUpness(status)
			logrus.WithField("up", isUp).Info("[Loop] Connection status changed")
			if changed {
				if status {
					if ctx != nil {
						logrus.Debug("[Loop] Canceling down action")
						<-ctx.Done()
					}
				} else {
					c, cancel := context.WithCancel(context.Background())
					ctx = c
					logrus.WithField("da", l.DownAction).Debug("[Loop] Starting DownAction")
					go l.DownAction.Run(ctx, cancel)
				}
			}
		} else {
			logrus.WithField("err", err).Error("[Loop] Error")
		}
		sleepTime := l.Delays[isUp]
		logrus.WithField("wait", sleepTime.Seconds()).Debugf("[Loop] Waiting for next loop iteration")
		time.Sleep(sleepTime)
	}
}
