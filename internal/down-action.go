package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/google/shlex"
	"github.com/sirupsen/logrus"
)

type DownAction struct {
	After        time.Duration
	Every        time.Duration
	BackoffLimit time.Duration
	Exec         string
	StopExec     string
}

type DownActionLoop struct {
	Da         *DownAction
	It         *DaIteration
	cancelFunc context.CancelFunc
}

type DaIteration struct {
	Iteration    int
	SleepTime    time.Duration
	LimitReached bool
}

const BackoffFactor = 1.5

// Only return an error if the command cannot be run.
func (dal *DownActionLoop) Execute(execString string) error {
	if execString == "" {
		return errors.New("no command to execute")
	}
	command, errSh := shlex.Split(execString)
	if errSh != nil {
		return fmt.Errorf("failed to parse DownAction definition: %w", errSh)
	}
	cmd := exec.Command(command[0], command[1:]...) // #nosec G204
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("UPD_ITERATION=%d", dal.It.Iteration))
	logrus.WithField("exec", execString).Debugf("[DownAction] Executing %s", execString)
	err := cmd.Start()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"exec": execString,
			"err":  err,
		}).Error("[DownAction] Failed to run")
		return fmt.Errorf("failed to execute DownAction: %w", err)
	}
	go func() {
		err := cmd.Wait()
		logrus.WithFields(logrus.Fields{
			"command": execString,
			"err":     err,
		}).Warn("[DownAction] Error")
	}()
	return nil
}

func NewDaIteration() *DaIteration {
	return &DaIteration{ //nolint:exhaustruct
		Iteration: -1,
	}
}

func (dal *DownActionLoop) iterate() {
	dal.It.Iteration++
	switch dal.It.Iteration {
	case 0:
		dal.It.SleepTime = dal.Da.After
	case 1:
		dal.It.SleepTime = dal.Da.Every
	default:
		if !dal.It.LimitReached {
			dal.It.SleepTime = time.Duration(BackoffFactor * float64(dal.It.SleepTime))
			if dal.Da.BackoffLimit != 0 && dal.It.SleepTime >= dal.Da.BackoffLimit {
				dal.It.SleepTime = dal.Da.BackoffLimit
				dal.It.LimitReached = true
			}
		}
	}
	logrus.WithFields(logrus.Fields{
		"iteration":    dal.It.Iteration,
		"sleepTime":    dal.It.SleepTime,
		"limitReached": dal.It.LimitReached,
	}).Debug("[DownAction] Iteration details")
}

func (dal *DownActionLoop) run(ctx context.Context) {
	dal.iterate()
	for {
		select {
		case <-ctx.Done():
			logrus.Debug("[DownAction] canceled")
			return
		case <-time.After(dal.It.SleepTime):
		}
		err := dal.Execute(dal.Da.Exec)
		if err != nil {
			logrus.WithField("err", err).Error("[DownAction] failed to execute")
		}
		if dal.Da.Every > 0 {
			dal.iterate()
		} else {
			break
		}
	}
}

func (da *DownAction) NewDownActionLoop() (*DownActionLoop, context.Context) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	dal := &DownActionLoop{
		Da:         da,
		It:         NewDaIteration(),
		cancelFunc: cancelFunc,
	}
	return dal, ctx
}

func (da *DownAction) Start() *DownActionLoop {
	logrus.SetLevel(logrus.DebugLevel)
	dal, ctx := da.NewDownActionLoop()
	logrus.Debug("[DownAction] kicking off run loop")
	go dal.run(ctx)
	return dal
}

func (dal *DownActionLoop) Stop() {
	_ = dal.Execute(dal.Da.StopExec) //nolint:errcheck
	logrus.Debug("[DownAction] sending shutdown signal")
	dal.cancelFunc()
}
