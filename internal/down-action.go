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

type DaIteration struct {
	iteration    int
	sleepTime    time.Duration
	limitReached bool
}

type DownActionLoop struct {
	da         *DownAction
	it         *DaIteration
	cancelFunc context.CancelFunc
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
	cmd.Env = append(cmd.Env, fmt.Sprintf("UPD_ITERATION=%d", dal.it.iteration))
	logrus.WithField("exec", cmd.String()).Info("[DownAction] executing command")
	err := cmd.Start()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"exec": cmd.String(),
			"err":  err,
		}).Error("[DownAction] failed to run")
		return fmt.Errorf("failed to execute DownAction: %w", err)
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"exec": cmd.String(),
				"err":  err,
			}).Warn("[DownAction] error executing command")
		}
	}()
	return nil
}

func NewDaIteration() *DaIteration {
	return &DaIteration{ //nolint:exhaustruct
		iteration: -1,
	}
}

func (dal *DownActionLoop) iterate() {
	dal.it.iteration++
	switch dal.it.iteration {
	case 0:
		dal.it.sleepTime = dal.da.After
	case 1:
		dal.it.sleepTime = dal.da.Every
	default:
		if !dal.it.limitReached {
			dal.it.sleepTime = time.Duration(BackoffFactor * float64(dal.it.sleepTime))
			if dal.da.BackoffLimit != 0 && dal.it.sleepTime >= dal.da.BackoffLimit {
				dal.it.sleepTime = dal.da.BackoffLimit
				dal.it.limitReached = true
			}
		}
	}
	logrus.WithFields(logrus.Fields{
		"iteration":    dal.it.iteration,
		"sleepTime":    dal.it.sleepTime,
		"limitReached": dal.it.limitReached,
	}).Debug("[DownAction] iteration details")
}

func (dal *DownActionLoop) run(ctx context.Context) {
	dal.iterate()
	for {
		select {
		case <-ctx.Done():
			logrus.Debug("[DownAction] canceled")
			return
		case <-time.After(dal.it.sleepTime):
		}
		err := dal.Execute(dal.da.Exec)
		if err != nil {
			logrus.WithField("err", err).Error("[DownAction] failed to execute")
		}
		if dal.da.Every > 0 {
			dal.iterate()
		} else {
			break
		}
	}
}

func (da *DownAction) NewDownActionLoop() (*DownActionLoop, context.Context) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	dal := &DownActionLoop{
		da:         da,
		it:         NewDaIteration(),
		cancelFunc: cancelFunc,
	}
	return dal, ctx
}

func (da *DownAction) Start() *DownActionLoop {
	dal, ctx := da.NewDownActionLoop()
	logrus.Debug("[DownAction] kicking off run loop")
	go dal.run(ctx)
	return dal
}

func (dal *DownActionLoop) Stop() {
	_ = dal.Execute(dal.da.StopExec) //nolint:errcheck
	logrus.Debug("[DownAction] sending shutdown signal")
	dal.cancelFunc()
}
