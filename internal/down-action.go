package internal

import (
	"context"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
)

type DownAction struct {
	After      time.Duration
	Every      time.Duration
	Exec       string
	ExecArgs   []string
	running    bool
	ctx        context.Context
	cancelFunc context.CancelFunc
}

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
		return err
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

func logSleep(sleepTime time.Duration) {
	logrus.WithField("sleep", sleepTime).Debug("[DownAction] Waiting")
}

func (da *DownAction) isRunning() bool {
	return da.running
}

func (da *DownAction) run() {
	da.running = true
	sleepTime := da.After
	logSleep(sleepTime)
	for {
		select {
		case <-da.ctx.Done():
			logrus.Debug("[DownAction] canceled")
			da.running = false
			return
		case <-time.After(sleepTime):
		}
		_ = da.Execute()
		if da.Every > 0 {
			sleepTime = da.Every
			logSleep(sleepTime)
		} else {
			break
		}
	}
}

func (da *DownAction) Start() {
	logrus.SetLevel(logrus.DebugLevel)
	da.ctx, da.cancelFunc = context.WithCancel(context.Background())
	logrus.Debug("[DownAction] kicking off run loop")
	go da.run()
}

func (da *DownAction) Stop() {
	if da.ctx != nil {
		logrus.Debug("[DownAction] sending shutdown signal")
		da.cancelFunc()
	}
}
